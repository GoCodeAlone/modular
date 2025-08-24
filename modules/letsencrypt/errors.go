package letsencrypt

import "errors"

// Static error definitions for the letsencrypt module
var (
	// Configuration errors
	ErrEmailRequired              = errors.New("email address is required for Let's Encrypt registration")
	ErrDomainsRequired            = errors.New("at least one domain is required")
	ErrConflictingProviders       = errors.New("cannot specify both HTTP and DNS challenge providers")
	ErrServerNameEmpty            = errors.New("server name is empty")
	ErrNoCertificateFound         = errors.New("no certificate found for domain")
	ErrCloudflareConfigMissing    = errors.New("cloudflare provider configuration is missing")
	ErrRoute53ConfigMissing       = errors.New("route53 provider configuration is missing")
	ErrDigitalOceanConfigMissing  = errors.New("digitalocean provider configuration is missing")
	ErrDigitalOceanTokenRequired  = errors.New("digitalocean auth token is required")
	ErrDNSConfigMissing           = errors.New("DNS provider configuration is missing or invalid")
	ErrUnsupportedDNSProvider     = errors.New("unsupported DNS provider")
	ErrGoogleCloudProjectRequired = errors.New("google Cloud DNS project ID is required")
	ErrAzureDNSConfigIncomplete   = errors.New("azure DNS provider requires client_id, client_secret, subscription_id, tenant_id, and resource_group")
	ErrNamecheapConfigIncomplete  = errors.New("namecheap DNS provider requires api_user, api_key, and username")
	ErrHTTPChallengeNotConfigured = errors.New("HTTP challenge handler not configured")

	// Certificate errors
	ErrCertificateFileNotFound = errors.New("certificate file not found")
	ErrKeyFileNotFound         = errors.New("key file not found")
	ErrPEMDecodeFailure        = errors.New("failed to decode PEM block containing certificate")

	// Event observation errors
	ErrNoSubjectForEventEmission = errors.New("no subject available for event emission")
)
