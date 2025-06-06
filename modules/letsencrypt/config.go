// Package letsencrypt provides a module for automatic SSL certificate generation
// via Let's Encrypt for the modular framework.
package letsencrypt

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// LetsEncryptConfig defines the configuration for the Let's Encrypt module.
type LetsEncryptConfig struct {
	// Email is the email address to use for registration with Let's Encrypt
	Email string `yaml:"email" json:"email"`

	// Domains is a list of domain names to obtain certificates for
	Domains []string `yaml:"domains" json:"domains"`

	// UseStaging determines whether to use Let's Encrypt's staging environment
	// Set to true for testing to avoid rate limits
	UseStaging bool `yaml:"use_staging" json:"use_staging"`

	// UseProduction is the opposite of UseStaging, for clarity in configuration
	UseProduction bool `yaml:"use_production" json:"use_production"`

	// StoragePath is the directory where certificates and account information will be stored
	StoragePath string `yaml:"storage_path" json:"storage_path"`

	// RenewBefore sets how long before expiry certificates should be renewed (in days)
	RenewBefore int `yaml:"renew_before" json:"renew_before"`

	// RenewBeforeDays is an alias for RenewBefore for backward compatibility
	RenewBeforeDays int `yaml:"renew_before_days" json:"renew_before_days"`

	// AutoRenew enables automatic certificate renewal
	AutoRenew bool `yaml:"auto_renew" json:"auto_renew"`

	// UseDNS indicates whether to use DNS challenges instead of HTTP
	UseDNS bool `yaml:"use_dns" json:"use_dns"`

	// DNSProvider configuration for DNS challenges
	DNSProvider *DNSProviderConfig `yaml:"dns_provider,omitempty" json:"dns_provider,omitempty"`

	// DNSConfig is a map of DNS provider specific configuration parameters
	DNSConfig map[string]string `yaml:"dns_config,omitempty" json:"dns_config,omitempty"`

	// HTTPProvider configuration for HTTP challenges
	HTTPProvider *HTTPProviderConfig `yaml:"http_provider,omitempty" json:"http_provider,omitempty"`

	// HTTPChallengeHandler is an HTTP handler for HTTP-01 challenges
	HTTPChallengeHandler http.Handler `yaml:"-" json:"-"`

	// CustomCACertificate is a custom CA certificate to be trusted
	CustomCACertificate []byte `yaml:"-" json:"-"`
}

// DNSProviderConfig defines the configuration for DNS challenge providers
type DNSProviderConfig struct {
	// Provider is the name of the DNS provider (e.g., "cloudflare", "route53", etc.)
	Provider string `yaml:"provider" json:"provider"`

	// Parameters is a map of provider-specific configuration parameters
	Parameters map[string]string `yaml:"parameters" json:"parameters"`

	// Provider-specific configurations
	Cloudflare   *CloudflareConfig   `yaml:"cloudflare,omitempty" json:"cloudflare,omitempty"`
	Route53      *Route53Config      `yaml:"route53,omitempty" json:"route53,omitempty"`
	DigitalOcean *DigitalOceanConfig `yaml:"digitalocean,omitempty" json:"digitalocean,omitempty"`
}

// CloudflareConfig holds the configuration for Cloudflare DNS API
type CloudflareConfig struct {
	Email    string `yaml:"email" json:"email"`
	APIKey   string `yaml:"api_key" json:"api_key"`
	APIToken string `yaml:"api_token" json:"api_token"`
}

// Route53Config holds the configuration for AWS Route53 DNS API
type Route53Config struct {
	AccessKeyID     string `yaml:"access_key_id" json:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key" json:"secret_access_key"`
	Region          string `yaml:"region" json:"region"`
	HostedZoneID    string `yaml:"hosted_zone_id" json:"hosted_zone_id"`
}

// DigitalOceanConfig holds the configuration for DigitalOcean DNS API
type DigitalOceanConfig struct {
	AuthToken string `yaml:"auth_token" json:"auth_token"`
}

// HTTPProviderConfig defines the configuration for HTTP challenge providers
type HTTPProviderConfig struct {
	// Use the built-in HTTP server for challenges
	UseBuiltIn bool `yaml:"use_built_in" json:"use_built_in"`

	// Port to use for the HTTP challenge server (default: 80)
	Port int `yaml:"port" json:"port"`
}

// Validate checks if the configuration is valid and sets default values
// where appropriate.
func (c *LetsEncryptConfig) Validate() error {
	// Email address is required
	if c.Email == "" {
		return ErrEmailRequired
	}

	// At least one domain is required
	if len(c.Domains) == 0 {
		return ErrDomainsRequired
	}

	// Set default storage path if not specified
	if c.StoragePath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory for default storage path: %w", err)
		}
		c.StoragePath = filepath.Join(homeDir, ".letsencrypt")
	}

	// Ensure the storage path exists
	if err := os.MkdirAll(c.StoragePath, 0700); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Set default renewal period if not specified
	if c.RenewBefore <= 0 {
		c.RenewBefore = 30 // 30 days
	}

	// Validate HTTP provider config
	if c.HTTPProvider != nil {
		if c.HTTPProvider.Port <= 0 {
			c.HTTPProvider.Port = 80
		}
	}

	// If both HTTP and DNS providers are specified, that's ambiguous
	if c.HTTPProvider != nil && c.DNSProvider != nil {
		return ErrConflictingProviders
	}

	// If no provider is specified, default to HTTP with built-in server
	if c.HTTPProvider == nil && c.DNSProvider == nil {
		c.HTTPProvider = &HTTPProviderConfig{
			UseBuiltIn: true,
			Port:       80,
		}
	}

	return nil
}
