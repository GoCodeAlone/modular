package modular

import (
	"fmt"
	"time"
)

// SecretProvider defines the interface for secure secret storage implementations.
// Different providers can offer varying levels of security, from basic obfuscation
// to hardware-backed secure memory handling.
type SecretProvider interface {
	// Name returns the provider's identifier
	Name() string

	// IsSecure indicates if this provider offers cryptographically secure memory handling
	IsSecure() bool

	// Store securely stores a secret value and returns a handle for retrieval
	Store(value string, secretType SecretType) (SecretHandle, error)

	// Retrieve retrieves the secret value using the provided handle
	Retrieve(handle SecretHandle) (string, error)

	// Destroy securely destroys the secret associated with the handle
	Destroy(handle SecretHandle) error

	// Compare performs a secure comparison of the secret with a provided value
	// This should use constant-time comparison to prevent timing attacks
	Compare(handle SecretHandle, value string) (bool, error)

	// IsEmpty checks if the secret handle represents an empty/null secret
	IsEmpty(handle SecretHandle) bool

	// Clone creates a new handle with the same secret value
	Clone(handle SecretHandle) (SecretHandle, error)

	// GetMetadata returns metadata about the secret (type, creation time, etc.)
	GetMetadata(handle SecretHandle) (SecretMetadata, error)

	// Cleanup performs any necessary cleanup operations (called on shutdown)
	Cleanup() error
}

// SecretHandle is an opaque reference to a stored secret.
// The actual implementation varies by provider.
type SecretHandle interface {
	// ID returns a unique identifier for this handle
	ID() string

	// Provider returns the name of the provider that created this handle
	Provider() string

	// IsValid returns true if this handle is still valid
	IsValid() bool
}

// SecretMetadata contains metadata about a secret
type SecretMetadata struct {
	Type          SecretType `json:"type"`
	Created       time.Time  `json:"created"`
	IsEmpty       bool       `json:"is_empty"`
	Provider      string     `json:"provider"`
	SecureStorage bool       `json:"secure_storage"`
}

// SecretProviderConfig configures secret provider behavior
type SecretProviderConfig struct {
	// Provider specifies which secret provider to use
	// Available options: "insecure", "memguard"
	Provider string `yaml:"provider" env:"SECRET_PROVIDER" default:"insecure" desc:"Secret storage provider (insecure, memguard)"`

	// EnableSecureMemory forces the use of secure memory providers only
	// If true and the configured provider is not secure, initialization will fail
	EnableSecureMemory bool `yaml:"enable_secure_memory" env:"ENABLE_SECURE_MEMORY" default:"false" desc:"Require secure memory handling"`

	// WarnOnInsecure logs warnings when using insecure providers
	WarnOnInsecure bool `yaml:"warn_on_insecure" env:"WARN_ON_INSECURE" default:"true" desc:"Warn when using insecure secret providers"`

	// MaxSecrets limits the number of secrets that can be stored (0 = unlimited)
	MaxSecrets int `yaml:"max_secrets" env:"MAX_SECRETS" default:"1000" desc:"Maximum number of secrets to store (0 = unlimited)"`

	// AutoDestroy automatically destroys secrets after the specified duration (0 = never)
	AutoDestroy time.Duration `yaml:"auto_destroy" env:"AUTO_DESTROY" default:"0s" desc:"Automatically destroy secrets after duration (0 = never)"`
}

// SecretProviderFactory creates secret providers based on configuration
type SecretProviderFactory struct {
	providers map[string]func(config SecretProviderConfig) (SecretProvider, error)
	logger    Logger
}

// NewSecretProviderFactory creates a new secret provider factory
func NewSecretProviderFactory(logger Logger) *SecretProviderFactory {
	factory := &SecretProviderFactory{
		providers: make(map[string]func(config SecretProviderConfig) (SecretProvider, error)),
		logger:    logger,
	}

	// Register built-in providers
	factory.RegisterProvider("insecure", NewInsecureSecretProvider)
	factory.RegisterProvider("memguard", NewMemguardSecretProvider)

	return factory
}

// RegisterProvider registers a custom secret provider
func (f *SecretProviderFactory) RegisterProvider(name string, creator func(config SecretProviderConfig) (SecretProvider, error)) {
	f.providers[name] = creator
}

// CreateProvider creates a secret provider based on configuration
func (f *SecretProviderFactory) CreateProvider(config SecretProviderConfig) (SecretProvider, error) {
	creator, exists := f.providers[config.Provider]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrUnknownSecretProvider, config.Provider)
	}

	provider, err := creator(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret provider %s: %w", config.Provider, err)
	}

	// Validate security requirements
	if config.EnableSecureMemory && !provider.IsSecure() {
		return nil, fmt.Errorf("%w: %s", ErrSecretProviderNotSecure, config.Provider)
	}

	// Log warning for insecure providers
	if config.WarnOnInsecure && !provider.IsSecure() && f.logger != nil {
		f.logger.Warn("Using insecure secret provider",
			"provider", provider.Name(),
			"recommendation", "Consider using 'memguard' provider for production")
	}

	return provider, nil
}

// ListProviders returns the names of all registered providers
func (f *SecretProviderFactory) ListProviders() []string {
	names := make([]string, 0, len(f.providers))
	for name := range f.providers {
		names = append(names, name)
	}
	return names
}

// GetProviderInfo returns information about a provider's security level
func (f *SecretProviderFactory) GetProviderInfo(name string) (map[string]interface{}, error) {
	creator, exists := f.providers[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrUnknownProvider, name)
	}

	// Create a temporary provider to get info
	tempProvider, err := creator(SecretProviderConfig{Provider: name})
	if err != nil {
		return nil, fmt.Errorf("failed to create provider for info: %w", err)
	}
	defer func() {
		if err := tempProvider.Cleanup(); err != nil {
			// Log cleanup error but don't fail the operation
			if f.logger != nil {
				f.logger.Warn("Failed to cleanup temp provider", "error", err)
			}
		}
	}()

	return map[string]interface{}{
		"name":      tempProvider.Name(),
		"secure":    tempProvider.IsSecure(),
		"available": true,
	}, nil
}

// Global secret provider factory and current provider
var (
	globalSecretProviderFactory *SecretProviderFactory
	globalSecretProvider        SecretProvider
)

// InitializeSecretProvider initializes the global secret provider
func InitializeSecretProvider(config SecretProviderConfig, logger Logger) error {
	if globalSecretProviderFactory == nil {
		globalSecretProviderFactory = NewSecretProviderFactory(logger)
	}

	provider, err := globalSecretProviderFactory.CreateProvider(config)
	if err != nil {
		return fmt.Errorf("failed to initialize secret provider: %w", err)
	}

	// Clean up previous provider if it exists
	if globalSecretProvider != nil {
		if err := globalSecretProvider.Cleanup(); err != nil {
			// Log cleanup error but continue with initialization
			// In production, this might warrant more attention
			_ = err // acknowledge error but don't fail initialization
		}
	}

	globalSecretProvider = provider

	if logger != nil {
		logger.Info("Secret provider initialized",
			"provider", provider.Name(),
			"secure", provider.IsSecure())
	}

	return nil
}

// GetGlobalSecretProvider returns the current global secret provider
func GetGlobalSecretProvider() SecretProvider {
	if globalSecretProvider == nil {
		// Fallback to insecure provider if not initialized
		provider, _ := NewInsecureSecretProvider(SecretProviderConfig{})
		return provider
	}
	return globalSecretProvider
}

// RegisterSecretProvider registers a custom provider globally
func RegisterSecretProvider(name string, creator func(config SecretProviderConfig) (SecretProvider, error)) {
	if globalSecretProviderFactory == nil {
		globalSecretProviderFactory = NewSecretProviderFactory(nil)
	}
	globalSecretProviderFactory.RegisterProvider(name, creator)
}
