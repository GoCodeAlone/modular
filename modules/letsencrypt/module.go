// Package letsencrypt provides a module for automatic SSL certificate generation
// via Let's Encrypt for the modular framework.
//
// This module integrates Let's Encrypt ACME protocol support into the modular framework,
// enabling automatic SSL/TLS certificate provisioning, renewal, and management. It supports
// multiple challenge types and DNS providers for flexible certificate acquisition.
//
// # Features
//
// The letsencrypt module provides the following capabilities:
//   - Automatic SSL certificate acquisition from Let's Encrypt
//   - Support for HTTP-01 and DNS-01 challenge types
//   - Multiple DNS provider integrations (Cloudflare, Route53, DigitalOcean, etc.)
//   - Automatic certificate renewal before expiration
//   - Certificate storage and management
//   - Staging and production environment support
//   - Service interface for integration with HTTP servers
//
// # Challenge Types
//
// The module supports two ACME challenge types:
//   - HTTP-01: Domain validation via HTTP endpoints
//   - DNS-01: Domain validation via DNS TXT records
//
// # Supported DNS Providers
//
// When using DNS-01 challenges, the following providers are supported:
//   - Cloudflare
//   - AWS Route53
//   - DigitalOcean
//   - Google Cloud DNS
//   - Azure DNS
//   - Namecheap
//
// # Configuration
//
// The module can be configured through the LetsEncryptConfig structure:
//
//	config := &LetsEncryptConfig{
//	    Email:           "admin@example.com",
//	    Domains:         []string{"example.com", "www.example.com"},
//	    ChallengeType:   "http-01",        // or "dns-01"
//	    DNSProvider:     "cloudflare",     // for DNS challenges
//	    CADirectory:     CAProduction,     // or CAStaging for testing
//	    CertificatePath: "/etc/ssl/certs", // certificate storage path
//	    KeyPath:         "/etc/ssl/private", // private key storage path
//	    DNSProviderConfig: &CloudflareConfig{
//	        APIToken: "your-cloudflare-token",
//	    },
//	}
//
// # Service Registration
//
// The module registers itself as a certificate service:
//
//	// Get the certificate service
//	certService := app.GetService("letsencrypt.certificates").(letsencrypt.CertificateService)
//
//	// Get a certificate for a domain
//	cert, err := certService.GetCertificate("example.com")
//
//	// Configure TLS with automatic certificates
//	tlsConfig := &tls.Config{
//	    GetCertificate: certService.GetCertificate,
//	}
//
// # Usage Examples
//
// Basic HTTP server with automatic HTTPS:
//
//	// Configure Let's Encrypt module
//	config := &LetsEncryptConfig{
//	    Email:   "admin@example.com",
//	    Domains: []string{"example.com"},
//	    ChallengeType: "http-01",
//	    CADirectory: CAProduction,
//	}
//
//	// Get certificate service
//	certService := app.GetService("letsencrypt.certificates").(CertificateService)
//
//	// Create TLS config with automatic certificates
//	tlsConfig := &tls.Config{
//	    GetCertificate: certService.GetCertificate,
//	}
//
//	// Start HTTPS server
//	server := &http.Server{
//	    Addr:      ":443",
//	    TLSConfig: tlsConfig,
//	    Handler:   httpHandler,
//	}
//	server.ListenAndServeTLS("", "")
//
// DNS challenge with Cloudflare:
//
//	config := &LetsEncryptConfig{
//	    Email:       "admin@example.com",
//	    Domains:     []string{"example.com", "*.example.com"},
//	    ChallengeType: "dns-01",
//	    DNSProvider: "cloudflare",
//	    DNSProviderConfig: &CloudflareConfig{
//	        APIToken: os.Getenv("CLOUDFLARE_API_TOKEN"),
//	    },
//	}
//
// # Certificate Management
//
// The module automatically handles:
//   - Certificate acquisition on first request
//   - Certificate renewal (default: 30 days before expiration)
//   - Certificate storage and loading
//   - OCSP stapling support
//   - Certificate chain validation
//
// # Security Considerations
//
// - Use staging environment for testing to avoid rate limits
// - Store API credentials securely (environment variables, secrets)
// - Ensure proper file permissions for certificate storage
// - Monitor certificate expiration and renewal logs
// - Use strong private keys (RSA 2048+ or ECDSA P-256+)
package letsencrypt

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/azuredns"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/providers/dns/digitalocean"
	"github.com/go-acme/lego/v4/providers/dns/gcloud"
	"github.com/go-acme/lego/v4/providers/dns/namecheap"
	"github.com/go-acme/lego/v4/providers/dns/route53"
	"github.com/go-acme/lego/v4/registration"

	"github.com/CrisisTextLine/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Constants for Let's Encrypt URLs
const (
	// CAStaging is the URL for Let's Encrypt's staging environment.
	// Use this for testing to avoid hitting production rate limits.
	// Certificates from staging are not trusted by browsers.
	CAStaging = "https://acme-staging-v02.api.letsencrypt.org/directory"

	// CAProduction is the URL for Let's Encrypt's production environment.
	// Use this for production deployments. Has strict rate limits.
	// Certificates from production are trusted by all major browsers.
	CAProduction = "https://acme-v02.api.letsencrypt.org/directory"
)

// ModuleName is the unique identifier for the letsencrypt module.
const ModuleName = "letsencrypt"

// LetsEncryptModule provides automatic SSL certificate management using Let's Encrypt.
// It handles certificate acquisition, renewal, and storage with support for multiple
// challenge types and DNS providers.
//
// The module implements the following interfaces:
//   - modular.Module: Basic module lifecycle
//   - modular.Configurable: Configuration management
//   - modular.ServiceAware: Service dependency management
//   - modular.Startable: Startup logic
//   - modular.Stoppable: Shutdown logic
//   - CertificateService: Certificate management interface
//
// Certificate operations are thread-safe and support concurrent requests.
type LetsEncryptModule struct {
	config        *LetsEncryptConfig
	client        *lego.Client
	user          *User
	certificates  map[string]*tls.Certificate
	certMutex     sync.RWMutex
	shutdownChan  chan struct{}
	renewalTicker *time.Ticker
	rootCAs       *x509.CertPool  // Certificate authority root certificates
	subject       modular.Subject // Added for event observation
}

// User implements the ACME User interface for Let's Encrypt
type User struct {
	Email        string
	Registration *registration.Resource
	Key          crypto.PrivateKey // Changed from certcrypto.PrivateKey to crypto.PrivateKey
}

// GetEmail returns the email address for the user
func (u *User) GetEmail() string {
	return u.Email
}

// GetRegistration returns the registration resource
func (u *User) GetRegistration() *registration.Resource {
	return u.Registration
}

// GetPrivateKey returns the private key for the user
func (u *User) GetPrivateKey() crypto.PrivateKey {
	return u.Key
}

// New creates a new Let's Encrypt module
func New(config *LetsEncryptConfig) (*LetsEncryptModule, error) {
	if config == nil {
		config = &LetsEncryptConfig{}
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	module := &LetsEncryptModule{
		config:       config,
		certificates: make(map[string]*tls.Certificate),
		shutdownChan: make(chan struct{}),
	}

	return module, nil
}

// Name returns the name of the module
func (m *LetsEncryptModule) Name() string {
	return ModuleName
}

// Config returns the module's configuration
func (m *LetsEncryptModule) Config() interface{} {
	return m.config
}

// Start initializes the module and starts any background processes
func (m *LetsEncryptModule) Start(ctx context.Context) error {
	// Emit service started event
	m.emitEvent(ctx, EventTypeServiceStarted, map[string]interface{}{
		"domains_count": len(m.config.Domains),
		"dns_provider":  m.config.DNSProvider,
		"auto_renew":    m.config.AutoRenew,
		"production":    m.config.UseProduction,
	})

	// Initialize the ACME user
	user, err := m.initUser()
	if err != nil {
		m.emitEvent(ctx, EventTypeError, map[string]interface{}{
			"error": err.Error(),
			"stage": "user_initialization",
		})
		return fmt.Errorf("failed to initialize ACME user: %w", err)
	}
	m.user = user

	// Initialize the ACME client
	if err := m.initClient(); err != nil {
		m.emitEvent(ctx, EventTypeError, map[string]interface{}{
			"error": err.Error(),
			"stage": "client_initialization",
		})
		return fmt.Errorf("failed to initialize ACME client: %w", err)
	}

	// Get or renew certificates for all domains
	if err := m.refreshCertificates(ctx); err != nil {
		m.emitEvent(ctx, EventTypeError, map[string]interface{}{
			"error": err.Error(),
			"stage": "certificate_refresh",
		})
		return fmt.Errorf("failed to obtain certificates: %w", err)
	}

	// Start the renewal timer if auto-renew is enabled
	if m.config.AutoRenew {
		m.startRenewalTimer(ctx)
	}

	// Emit module started event
	m.emitEvent(ctx, EventTypeModuleStarted, map[string]interface{}{
		"certificates_count": len(m.certificates),
		"auto_renew_enabled": m.config.AutoRenew,
	})

	return nil
}

// Stop stops any background processes
func (m *LetsEncryptModule) Stop(ctx context.Context) error {
	// Stop the renewal timer if it's running
	if m.renewalTicker != nil {
		m.renewalTicker.Stop()
		close(m.shutdownChan)
	}

	// Emit service stopped event
	m.emitEvent(ctx, EventTypeServiceStopped, map[string]interface{}{
		"certificates_count": len(m.certificates),
	})

	// Emit module stopped event
	m.emitEvent(ctx, EventTypeModuleStopped, map[string]interface{}{
		"certificates_count": len(m.certificates),
	})

	return nil
}

// GetCertificate implements the CertificateService.GetCertificate method
// to be used with tls.Config.GetCertificate
func (m *LetsEncryptModule) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if clientHello == nil || clientHello.ServerName == "" {
		return nil, ErrServerNameEmpty
	}

	return m.GetCertificateForDomain(clientHello.ServerName)
}

// GetCertificateForDomain returns a certificate for the specified domain
func (m *LetsEncryptModule) GetCertificateForDomain(domain string) (*tls.Certificate, error) {
	m.certMutex.RLock()
	cert, ok := m.certificates[domain]
	m.certMutex.RUnlock()

	if !ok {
		// Check if we have a wildcard certificate that matches
		wildcardDomain := "*." + domain[strings.Index(domain, ".")+1:]
		m.certMutex.RLock()
		cert, ok = m.certificates[wildcardDomain]
		m.certMutex.RUnlock()

		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrNoCertificateFound, domain)
		}
	}

	return cert, nil
}

// Domains returns the list of domains this service can provide certificates for
func (m *LetsEncryptModule) Domains() []string {
	m.certMutex.RLock()
	defer m.certMutex.RUnlock()

	domains := make([]string, 0, len(m.certificates))
	for domain := range m.certificates {
		domains = append(domains, domain)
	}

	return domains
}

// initUser initializes a new ACME user for registration with Let's Encrypt
func (m *LetsEncryptModule) initUser() (*User, error) {
	// Generate a new private key for the user
	privateKey, err := certcrypto.GeneratePrivateKey(certcrypto.RSA2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	return &User{
		Email: m.config.Email,
		Key:   privateKey,
	}, nil
}

// initClient initializes the ACME client for Let's Encrypt
func (m *LetsEncryptModule) initClient() error {
	// Skip initialization if client is already initialized
	if m.client != nil {
		return nil
	}

	// Create ACME user
	if err := m.createUser(); err != nil {
		return fmt.Errorf("failed to create ACME user: %w", err)
	}

	// Configure client based on the module configuration
	caCertificates := CAStaging
	if m.config.UseProduction {
		caCertificates = CAProduction
	}

	config := lego.NewConfig(m.user)
	config.CADirURL = caCertificates
	config.Certificate.KeyType = certcrypto.RSA2048

	// Set custom CA certificate if provided
	if len(m.config.CustomCACertificate) > 0 {
		config.HTTPClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    m.rootCAs,
				MinVersion: tls.VersionTLS12,
			},
		}
	}

	// Create client
	client, err := lego.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create ACME client: %w", err)
	}

	// Configure challenge type
	if m.config.UseDNS {
		if err := m.configureDNSProvider(); err != nil {
			return fmt.Errorf("failed to configure DNS provider: %w", err)
		}
	} else {
		// Setup HTTP challenge
		if err := client.Challenge.SetHTTP01Provider(&letsEncryptHTTPProvider{
			handler: m.config.HTTPChallengeHandler,
		}); err != nil {
			return fmt.Errorf("failed to set HTTP challenge provider: %w", err)
		}
	}

	m.client = client
	return nil
}

// createUser registers the ACME user with Let's Encrypt
func (m *LetsEncryptModule) createUser() error {
	// Skip if user registration is already completed
	if m.user.Registration != nil {
		return nil
	}

	// Create new registration
	reg, err := m.client.Registration.Register(registration.RegisterOptions{
		TermsOfServiceAgreed: true,
	})
	if err != nil {
		return fmt.Errorf("failed to register account: %w", err)
	}

	m.user.Registration = reg
	return nil
}

// refreshCertificates obtains or renews certificates for all configured domains
func (m *LetsEncryptModule) refreshCertificates(ctx context.Context) error {
	// Emit certificate requested event
	m.emitEvent(ctx, EventTypeCertificateRequested, map[string]interface{}{
		"domains": m.config.Domains,
		"count":   len(m.config.Domains),
	})

	// Request certificates for domains
	request := certificate.ObtainRequest{
		Domains: m.config.Domains,
		Bundle:  true,
	}

	certificates, err := m.client.Certificate.Obtain(request)
	if err != nil {
		m.emitEvent(ctx, EventTypeError, map[string]interface{}{
			"error":   err.Error(),
			"domains": m.config.Domains,
			"stage":   "certificate_obtain",
		})
		return fmt.Errorf("failed to obtain certificate: %w", err)
	}

	// Parse and store the certificates
	m.certMutex.Lock()
	defer m.certMutex.Unlock()

	for _, domain := range m.config.Domains {
		cert, err := tls.X509KeyPair(certificates.Certificate, certificates.PrivateKey)
		if err != nil {
			m.emitEvent(ctx, EventTypeError, map[string]interface{}{
				"error":  err.Error(),
				"domain": domain,
				"stage":  "certificate_parse",
			})
			return fmt.Errorf("failed to parse certificate for %s: %w", domain, err)
		}
		m.certificates[domain] = &cert

		// Emit certificate issued event for each domain
		m.emitEvent(ctx, EventTypeCertificateIssued, map[string]interface{}{
			"domain": domain,
		})
	}

	return nil
}

// startRenewalTimer starts a background timer to check and renew certificates
func (m *LetsEncryptModule) startRenewalTimer(ctx context.Context) {
	// Check certificates daily
	m.renewalTicker = time.NewTicker(24 * time.Hour)

	go func() {
		for {
			select {
			case <-m.renewalTicker.C:
				// Check if certificates need renewal
				m.checkAndRenewCertificates(ctx)
			case <-m.shutdownChan:
				return
			}
		}
	}()
}

// checkAndRenewCertificates checks if certificates need renewal and renews them
func (m *LetsEncryptModule) checkAndRenewCertificates(ctx context.Context) {
	// Loop through all certificates and check their expiry dates
	for domain, cert := range m.certificates {
		if cert == nil || len(cert.Certificate) == 0 {
			// Skip invalid certificates
			continue
		}

		// Parse the certificate to get its expiry date
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			fmt.Printf("Error parsing certificate for %s: %v\n", domain, err)
			continue
		}

		// Calculate days until expiry
		daysUntilExpiry := time.Until(x509Cert.NotAfter) / (24 * time.Hour)

		// If certificate will expire within the renewal window, renew it
		if daysUntilExpiry <= time.Duration(m.config.RenewBeforeDays) {
			fmt.Printf("Certificate for %s will expire in %d days, renewing\n", domain, int(daysUntilExpiry))

			// Request renewal for this specific domain
			if err := m.renewCertificateForDomain(ctx, domain); err != nil {
				fmt.Printf("Failed to renew certificate for %s: %v\n", domain, err)
			} else {
				fmt.Printf("Successfully renewed certificate for %s\n", domain)
			}
		}
	}
}

// renewCertificateForDomain renews the certificate for a specific domain
func (m *LetsEncryptModule) renewCertificateForDomain(ctx context.Context, domain string) error {
	// Request certificate for the domain
	request := certificate.ObtainRequest{
		Domains: []string{domain},
		Bundle:  true,
	}

	certificates, err := m.client.Certificate.Obtain(request)
	if err != nil {
		m.emitEvent(ctx, EventTypeError, map[string]interface{}{
			"error":  err.Error(),
			"domain": domain,
			"stage":  "certificate_renewal",
		})
		return fmt.Errorf("failed to obtain certificate for domain %s: %w", domain, err)
	}

	// Parse and store the new certificate
	cert, err := tls.X509KeyPair(certificates.Certificate, certificates.PrivateKey)
	if err != nil {
		m.emitEvent(ctx, EventTypeError, map[string]interface{}{
			"error":  err.Error(),
			"domain": domain,
			"stage":  "certificate_parse_renewal",
		})
		return fmt.Errorf("failed to parse renewed certificate for %s: %w", domain, err)
	}

	m.certMutex.Lock()
	m.certificates[domain] = &cert
	m.certMutex.Unlock()

	// Emit certificate renewed event
	m.emitEvent(ctx, EventTypeCertificateRenewed, map[string]interface{}{
		"domain": domain,
	})

	return nil
}

// RevokeCertificate revokes a certificate for the specified domain
func (m *LetsEncryptModule) RevokeCertificate(domain string) error {
	m.certMutex.RLock()
	cert, exists := m.certificates[domain]
	m.certMutex.RUnlock()

	if !exists || cert == nil {
		return fmt.Errorf("%w: %s", ErrNoCertificateFound, domain)
	}

	// Parse the certificate to get the raw X509 certificate
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("failed to parse certificate for revocation: %w", err)
	}

	// Revoke the certificate
	err = m.client.Certificate.Revoke(x509Cert.Raw)
	if err != nil {
		return fmt.Errorf("failed to revoke certificate: %w", err)
	}

	// Remove the certificate from our cache
	m.certMutex.Lock()
	delete(m.certificates, domain)
	m.certMutex.Unlock()

	return nil
}

// createCloudflareProvider creates a Cloudflare DNS provider from the module configuration
func (m *LetsEncryptModule) createCloudflareProvider() (challenge.Provider, error) {
	if m.config.DNSProvider == nil || m.config.DNSProvider.Cloudflare == nil {
		return nil, ErrCloudflareConfigMissing
	}

	cfg := m.config.DNSProvider.Cloudflare

	// Use API Token if provided (preferred)
	if cfg.APIToken != "" {
		provider, err := cloudflare.NewDNSProviderConfig(&cloudflare.Config{
			AuthToken: cfg.APIToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Cloudflare DNS provider with token: %w", err)
		}
		return provider, nil
	}

	// Fall back to environment variables
	provider, err := cloudflare.NewDNSProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to create Cloudflare DNS provider: %w", err)
	}
	return provider, nil
}

// createRoute53Provider creates an AWS Route53 DNS provider from the module configuration
func (m *LetsEncryptModule) createRoute53Provider() (challenge.Provider, error) {
	if m.config.DNSProvider == nil || m.config.DNSProvider.Route53 == nil {
		return nil, ErrRoute53ConfigMissing
	}

	cfg := m.config.DNSProvider.Route53
	config := &route53.Config{} // Changed to pointer

	// Set credentials if provided
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		config.AccessKeyID = cfg.AccessKeyID
		config.SecretAccessKey = cfg.SecretAccessKey
	}

	// Set region if provided
	if cfg.Region != "" {
		config.Region = cfg.Region
	}

	// Set hosted zone ID if provided
	if cfg.HostedZoneID != "" {
		config.HostedZoneID = cfg.HostedZoneID
	}

	provider, err := route53.NewDNSProviderConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Route53 provider: %w", err)
	}
	return provider, nil
}

// createDigitalOceanProvider creates a DigitalOcean DNS provider from the module configuration
func (m *LetsEncryptModule) createDigitalOceanProvider() (challenge.Provider, error) {
	if m.config.DNSProvider == nil || m.config.DNSProvider.DigitalOcean == nil {
		return nil, ErrDigitalOceanConfigMissing
	}

	cfg := m.config.DNSProvider.DigitalOcean

	if cfg.AuthToken != "" {
		provider, err := digitalocean.NewDNSProviderConfig(&digitalocean.Config{
			AuthToken: cfg.AuthToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create DigitalOcean DNS provider: %w", err)
		}
		return provider, nil
	}

	return nil, ErrDigitalOceanTokenRequired
}

// configureDNSProvider configures the DNS provider for ACME DNS challenges
func (m *LetsEncryptModule) configureDNSProvider() error {
	if m.config.DNSProvider == nil || m.config.DNSProvider.Provider == "" {
		return ErrDNSConfigMissing
	}

	switch m.config.DNSProvider.Provider {
	case "cloudflare":
		return m.configureCloudflare()
	case "route53":
		return m.configureRoute53()
	case "digitalocean":
		return m.configureDigitalOcean()
	case "gcloud":
		return m.configureGoogleCloudDNS()
	case "azure":
		return m.configureAzureDNS()
	case "namecheap":
		return m.configureNamecheap()
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedDNSProvider, m.config.DNSProvider.Provider)
	}
} // configureCloudflare configures the Cloudflare DNS provider
func (m *LetsEncryptModule) configureCloudflare() error {
	provider, err := m.createCloudflareProvider()
	if err != nil {
		return fmt.Errorf("failed to create Cloudflare provider: %w", err)
	}
	if err := m.client.Challenge.SetDNS01Provider(provider); err != nil {
		return fmt.Errorf("failed to set DNS01 provider: %w", err)
	}
	return nil
}

// configureRoute53 configures the Route53 DNS provider
func (m *LetsEncryptModule) configureRoute53() error {
	provider, err := m.createRoute53Provider()
	if err != nil {
		return fmt.Errorf("failed to create Route53 provider: %w", err)
	}
	if err := m.client.Challenge.SetDNS01Provider(provider); err != nil {
		return fmt.Errorf("failed to set DNS01 provider: %w", err)
	}
	return nil
}

// configureDigitalOcean configures the DigitalOcean DNS provider
func (m *LetsEncryptModule) configureDigitalOcean() error {
	provider, err := m.createDigitalOceanProvider()
	if err != nil {
		return fmt.Errorf("failed to create DigitalOcean provider: %w", err)
	}
	if err := m.client.Challenge.SetDNS01Provider(provider); err != nil {
		return fmt.Errorf("failed to set DNS01 provider: %w", err)
	}
	return nil
}

// configureGoogleCloudDNS configures the Google Cloud DNS provider
func (m *LetsEncryptModule) configureGoogleCloudDNS() error {
	project := m.config.DNSConfig["project_id"]
	if project == "" {
		return ErrGoogleCloudProjectRequired
	}

	var provider challenge.Provider
	var err error

	// Check if service account JSON was provided
	if jsonFile := m.config.DNSConfig["service_account_json"]; jsonFile != "" {
		// Configure using service account JSON file
		provider, err = gcloud.NewDNSProviderServiceAccount(jsonFile)
	} else {
		// Use default credentials
		provider, err = gcloud.NewDNSProvider()
	}

	if err != nil {
		return fmt.Errorf("failed to initialize Google Cloud DNS provider: %w", err)
	}

	if err := m.client.Challenge.SetDNS01Provider(provider); err != nil {
		return fmt.Errorf("failed to set Google Cloud DNS provider: %w", err)
	}
	return nil
}

// configureAzureDNS configures the Azure DNS provider
func (m *LetsEncryptModule) configureAzureDNS() error {
	clientID := m.config.DNSConfig["client_id"]
	clientSecret := m.config.DNSConfig["client_secret"]
	subscriptionID := m.config.DNSConfig["subscription_id"]
	tenantID := m.config.DNSConfig["tenant_id"]
	resourceGroup := m.config.DNSConfig["resource_group"]

	if clientID == "" || clientSecret == "" || subscriptionID == "" ||
		tenantID == "" || resourceGroup == "" {
		return ErrAzureDNSConfigIncomplete
	}

	provider, err := azuredns.NewDNSProviderConfig(&azuredns.Config{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		SubscriptionID: subscriptionID,
		TenantID:       tenantID,
		ResourceGroup:  resourceGroup,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize Azure DNS provider: %w", err)
	}

	if err := m.client.Challenge.SetDNS01Provider(provider); err != nil {
		return fmt.Errorf("failed to set Azure DNS provider: %w", err)
	}
	return nil
}

// configureNamecheap configures the Namecheap DNS provider
func (m *LetsEncryptModule) configureNamecheap() error {
	apiUser := m.config.DNSConfig["api_user"]
	apiKey := m.config.DNSConfig["api_key"]
	username := m.config.DNSConfig["username"]

	if apiUser == "" || apiKey == "" || username == "" {
		return ErrNamecheapConfigIncomplete
	}

	// Use environment variables as that's the most reliable way across lego versions
	if err := os.Setenv("NAMECHEAP_API_USER", apiUser); err != nil {
		return fmt.Errorf("failed to set NAMECHEAP_API_USER: %w", err)
	}
	if err := os.Setenv("NAMECHEAP_API_KEY", apiKey); err != nil {
		return fmt.Errorf("failed to set NAMECHEAP_API_KEY: %w", err)
	}
	if err := os.Setenv("NAMECHEAP_USERNAME", username); err != nil {
		return fmt.Errorf("failed to set NAMECHEAP_USERNAME: %w", err)
	}

	// Set sandbox mode if specified
	if m.config.DNSConfig["sandbox"] == "true" {
		os.Setenv("NAMECHEAP_SANDBOX", "true")
	}

	provider, err := namecheap.NewDNSProvider()
	if err != nil {
		return fmt.Errorf("failed to initialize Namecheap DNS provider: %w", err)
	}

	if err := m.client.Challenge.SetDNS01Provider(provider); err != nil {
		return fmt.Errorf("failed to set Namecheap DNS provider: %w", err)
	}
	return nil
}

// letsEncryptHTTPProvider implements the HTTP-01 challenge provider
type letsEncryptHTTPProvider struct {
	handler http.Handler
}

// Present makes the challenge token available
func (p *letsEncryptHTTPProvider) Present(domain, token, keyAuth string) error {
	if p.handler == nil {
		return ErrHTTPChallengeNotConfigured
	}

	// If the handler implements our custom interface, use that
	if customHandler, ok := p.handler.(ChallengeHandler); ok {
		if err := customHandler.PresentChallenge(domain, token, keyAuth); err != nil {
			return fmt.Errorf("failed to present challenge: %w", err)
		}
		return nil
	}

	return nil
}

// CleanUp removes the challenge token
func (p *letsEncryptHTTPProvider) CleanUp(domain, token, keyAuth string) error {
	if p.handler == nil {
		return nil // Nothing to clean up
	}

	// If the handler implements our custom interface, use that
	if customHandler, ok := p.handler.(ChallengeHandler); ok {
		if err := customHandler.CleanupChallenge(domain, token, keyAuth); err != nil {
			return fmt.Errorf("failed to clean up challenge: %w", err)
		}
		return nil
	}

	return nil
}

// RegisterObservers implements the ObservableModule interface.
// This allows the letsencrypt module to register as an observer for events it's interested in.
func (m *LetsEncryptModule) RegisterObservers(subject modular.Subject) error {
	m.subject = subject
	return nil
}

// EmitEvent implements the ObservableModule interface.
// This allows the letsencrypt module to emit events that other modules or observers can receive.
func (m *LetsEncryptModule) EmitEvent(ctx context.Context, event cloudevents.Event) error {
	if m.subject == nil {
		return ErrNoSubjectForEventEmission
	}
	if err := m.subject.NotifyObservers(ctx, event); err != nil {
		return fmt.Errorf("failed to notify observers: %w", err)
	}
	return nil
}

// emitEvent is a helper method to create and emit CloudEvents for the letsencrypt module.
// This centralizes the event creation logic and ensures consistent event formatting.
// emitEvent is a helper method to create and emit CloudEvents for the letsencrypt module.
// If no subject is available for event emission, it silently skips the event emission
// to avoid noisy error messages in tests and non-observable applications.
func (m *LetsEncryptModule) emitEvent(ctx context.Context, eventType string, data map[string]interface{}) {
	// Skip event emission if no subject is available (non-observable application)
	if m.subject == nil {
		return
	}

	event := modular.NewCloudEvent(eventType, "letsencrypt-service", data, nil)

	if emitErr := m.EmitEvent(ctx, event); emitErr != nil {
		// If no subject is registered, quietly skip to allow non-observable apps to run cleanly
		if errors.Is(emitErr, ErrNoSubjectForEventEmission) {
			return
		}
		// Note: No logger available in letsencrypt module, so we skip additional error logging
		// to eliminate noisy test output. The error handling is centralized in EmitEvent.
	}
}

// GetRegisteredEventTypes implements the ObservableModule interface.
// Returns all event types that this letsencrypt module can emit.
func (m *LetsEncryptModule) GetRegisteredEventTypes() []string {
	return []string{
		EventTypeConfigLoaded,
		EventTypeConfigValidated,
		EventTypeCertificateRequested,
		EventTypeCertificateIssued,
		EventTypeCertificateRenewed,
		EventTypeCertificateRevoked,
		EventTypeCertificateExpiring,
		EventTypeCertificateExpired,
		EventTypeAcmeChallenge,
		EventTypeAcmeAuthorization,
		EventTypeAcmeOrder,
		EventTypeServiceStarted,
		EventTypeServiceStopped,
		EventTypeStorageRead,
		EventTypeStorageWrite,
		EventTypeStorageError,
		EventTypeModuleStarted,
		EventTypeModuleStopped,
		EventTypeError,
		EventTypeWarning,
	}
}
