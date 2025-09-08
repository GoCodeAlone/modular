package modular

import (
	"crypto/rand"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// InsecureSecretProvider implements SecretProvider using the original XOR-based approach.
// This provider offers basic obfuscation but NO cryptographic security guarantees.
//
// SECURITY WARNING: This provider:
// - Uses simple XOR encryption for obfuscation only
// - Cannot prevent memory dumps from revealing secrets
// - Cannot guarantee secure memory clearing in Go
// - Should NOT be used for highly sensitive secrets in production
//
// Use this provider for:
// - Development and testing environments
// - Non-critical secrets where convenience outweighs security
// - Situations where secure memory libraries are unavailable
type InsecureSecretProvider struct {
	name        string
	secrets     map[string]*insecureSecret
	mu          sync.RWMutex
	nextID      int64
	maxSecrets  int
	autoDestroy time.Duration
}

// insecureSecret represents a secret stored with XOR obfuscation
type insecureSecret struct {
	id             string
	encryptedValue []byte
	key            []byte
	metadata       SecretMetadata
}

// insecureHandle implements SecretHandle for the insecure provider
type insecureHandle struct {
	id       string
	provider string
	valid    bool
}

func (h *insecureHandle) ID() string {
	return h.id
}

func (h *insecureHandle) Provider() string {
	return h.provider
}

func (h *insecureHandle) IsValid() bool {
	return h.valid
}

// NewInsecureSecretProvider creates a new insecure secret provider
func NewInsecureSecretProvider(config SecretProviderConfig) (SecretProvider, error) {
	provider := &InsecureSecretProvider{
		name:        "insecure",
		secrets:     make(map[string]*insecureSecret),
		maxSecrets:  config.MaxSecrets,
		autoDestroy: config.AutoDestroy,
	}

	return provider, nil
}

func (p *InsecureSecretProvider) Name() string {
	return p.name
}

func (p *InsecureSecretProvider) IsSecure() bool {
	return false // This provider is not cryptographically secure
}

func (p *InsecureSecretProvider) Store(value string, secretType SecretType) (SecretHandle, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check max secrets limit
	if p.maxSecrets > 0 && len(p.secrets) >= p.maxSecrets {
		return nil, fmt.Errorf("%w: %d", ErrSecretLimitReached, p.maxSecrets)
	}

	// Generate unique ID
	p.nextID++
	id := fmt.Sprintf("insecure_%d_%d", time.Now().UnixNano(), p.nextID)

	handle := &insecureHandle{
		id:       id,
		provider: p.name,
		valid:    true,
	}

	// Handle empty secrets
	if value == "" {
		secret := &insecureSecret{
			id: id,
			metadata: SecretMetadata{
				Type:          secretType,
				Created:       time.Now(),
				IsEmpty:       true,
				Provider:      p.name,
				SecureStorage: false,
			},
		}
		p.secrets[id] = secret

		// Set up auto-destroy if configured
		if p.autoDestroy > 0 {
			go p.scheduleDestroy(id, p.autoDestroy)
		}

		return handle, nil
	}

	// Generate random key for XOR encryption
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		// Fallback to deterministic key if crypto/rand fails
		for i := range key {
			key[i] = byte(i * 7)
		}
	}

	// XOR encrypt the value (basic obfuscation)
	valueBytes := []byte(value)
	encrypted := make([]byte, len(valueBytes))
	for i, b := range valueBytes {
		encrypted[i] = b ^ key[i%len(key)]
	}

	secret := &insecureSecret{
		id:             id,
		encryptedValue: encrypted,
		key:            key,
		metadata: SecretMetadata{
			Type:          secretType,
			Created:       time.Now(),
			IsEmpty:       false,
			Provider:      p.name,
			SecureStorage: false,
		},
	}

	p.secrets[id] = secret

	// Set finalizer for cleanup
	runtime.SetFinalizer(secret, (*insecureSecret).zeroMemory)

	// Set up auto-destroy if configured
	if p.autoDestroy > 0 {
		go p.scheduleDestroy(id, p.autoDestroy)
	}

	return handle, nil
}

func (p *InsecureSecretProvider) Retrieve(handle SecretHandle) (string, error) {
	if handle == nil || !handle.IsValid() {
		return "", ErrInvalidSecretHandle
	}

	p.mu.RLock()
	secret, exists := p.secrets[handle.ID()]
	p.mu.RUnlock()

	if !exists {
		return "", ErrSecretNotFound
	}

	if secret.metadata.IsEmpty {
		return "", nil
	}

	// Decrypt using XOR
	decrypted := make([]byte, len(secret.encryptedValue))
	for i, b := range secret.encryptedValue {
		decrypted[i] = b ^ secret.key[i%len(secret.key)]
	}

	result := string(decrypted)

	// Zero out decrypted bytes (though this doesn't guarantee security in Go)
	for i := range decrypted {
		decrypted[i] = 0
	}

	return result, nil
}

func (p *InsecureSecretProvider) Destroy(handle SecretHandle) error {
	if handle == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	secret, exists := p.secrets[handle.ID()]
	if !exists {
		return nil // Already destroyed or never existed
	}

	// Zero memory and remove from map
	secret.zeroMemory()
	delete(p.secrets, handle.ID())

	// Invalidate handle
	if h, ok := handle.(*insecureHandle); ok {
		h.valid = false
	}

	return nil
}

func (p *InsecureSecretProvider) Compare(handle SecretHandle, value string) (bool, error) {
	if handle == nil || !handle.IsValid() {
		return value == "", nil
	}

	secretValue, err := p.Retrieve(handle)
	if err != nil {
		return false, err
	}

	// Use constant-time comparison to prevent timing attacks
	result := constantTimeEquals(secretValue, value)

	// Attempt to zero the retrieved value (limited effectiveness in Go)
	zeroString(&secretValue)

	return result, nil
}

func (p *InsecureSecretProvider) IsEmpty(handle SecretHandle) bool {
	if handle == nil || !handle.IsValid() {
		return true
	}

	p.mu.RLock()
	secret, exists := p.secrets[handle.ID()]
	p.mu.RUnlock()

	if !exists {
		return true
	}

	return secret.metadata.IsEmpty
}

func (p *InsecureSecretProvider) Clone(handle SecretHandle) (SecretHandle, error) {
	if handle == nil || !handle.IsValid() {
		return nil, ErrInvalidSecretHandle
	}

	p.mu.RLock()
	secret, exists := p.secrets[handle.ID()]
	p.mu.RUnlock()

	if !exists {
		return nil, ErrSecretNotFound
	}

	// Clone by retrieving and storing again
	if secret.metadata.IsEmpty {
		return p.Store("", secret.metadata.Type)
	}

	value, err := p.Retrieve(handle)
	if err != nil {
		return nil, err
	}

	newHandle, err := p.Store(value, secret.metadata.Type)

	// Zero out the retrieved value
	zeroString(&value)

	return newHandle, err
}

func (p *InsecureSecretProvider) GetMetadata(handle SecretHandle) (SecretMetadata, error) {
	if handle == nil || !handle.IsValid() {
		return SecretMetadata{}, ErrInvalidSecretHandle
	}

	p.mu.RLock()
	secret, exists := p.secrets[handle.ID()]
	p.mu.RUnlock()

	if !exists {
		return SecretMetadata{}, ErrSecretNotFound
	}

	return secret.metadata, nil
}

func (p *InsecureSecretProvider) Cleanup() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Zero and clear all secrets
	for id, secret := range p.secrets {
		secret.zeroMemory()
		delete(p.secrets, id)
	}

	return nil
}

// scheduleDestroy automatically destroys a secret after the specified duration
func (p *InsecureSecretProvider) scheduleDestroy(id string, delay time.Duration) {
	time.Sleep(delay)

	p.mu.Lock()
	secret, exists := p.secrets[id]
	if exists {
		secret.zeroMemory()
		delete(p.secrets, id)
	}
	p.mu.Unlock()
}

// zeroMemory zeros out the secret's memory
func (s *insecureSecret) zeroMemory() {
	if s == nil {
		return
	}

	// Zero encrypted value
	for i := range s.encryptedValue {
		s.encryptedValue[i] = 0
	}

	// Zero key
	for i := range s.key {
		s.key[i] = 0
	}

	// Clear slices
	s.encryptedValue = nil
	s.key = nil
}

// GetInsecureProviderStats returns statistics about the insecure provider (for testing/monitoring)
func GetInsecureProviderStats(provider SecretProvider) map[string]interface{} {
	if p, ok := provider.(*InsecureSecretProvider); ok {
		p.mu.RLock()
		defer p.mu.RUnlock()

		return map[string]interface{}{
			"active_secrets":  len(p.secrets),
			"max_secrets":     p.maxSecrets,
			"auto_destroy":    p.autoDestroy.String(),
			"provider_secure": p.IsSecure(),
		}
	}

	return map[string]interface{}{
		"error": "not an insecure provider",
	}
}
