// Package letsencrypt provides a module for automatic SSL certificate generation
// via Let's Encrypt for the modular framework.
package letsencrypt

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
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
)

// Constants for Let's Encrypt URLs
const (
	// CAStaging is the URL for Let's Encrypt's staging environment
	CAStaging = "https://acme-staging-v02.api.letsencrypt.org/directory"
	// CAProduction is the URL for Let's Encrypt's production environment
	CAProduction = "https://acme-v02.api.letsencrypt.org/directory"
)

// ModuleName is the name of this module
const ModuleName = "letsencrypt"

// LetsEncryptModule represents the Let's Encrypt module
type LetsEncryptModule struct {
	config        *LetsEncryptConfig
	client        *lego.Client
	user          *User
	certificates  map[string]*tls.Certificate
	certMutex     sync.RWMutex
	shutdownChan  chan struct{}
	renewalTicker *time.Ticker
	rootCAs       *x509.CertPool // Changed from *tls.CertPool to *x509.CertPool
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
	// Initialize the ACME user
	user, err := m.initUser()
	if err != nil {
		return fmt.Errorf("failed to initialize ACME user: %w", err)
	}
	m.user = user

	// Initialize the ACME client
	if err := m.initClient(); err != nil {
		return fmt.Errorf("failed to initialize ACME client: %w", err)
	}

	// Get or renew certificates for all domains
	if err := m.refreshCertificates(); err != nil {
		return fmt.Errorf("failed to obtain certificates: %w", err)
	}

	// Start the renewal timer if auto-renew is enabled
	if m.config.AutoRenew {
		m.startRenewalTimer()
	}

	return nil
}

// Stop stops any background processes
func (m *LetsEncryptModule) Stop(ctx context.Context) error {
	// Stop the renewal timer if it's running
	if m.renewalTicker != nil {
		m.renewalTicker.Stop()
		close(m.shutdownChan)
	}

	return nil
}

// GetCertificate implements the CertificateService.GetCertificate method
// to be used with tls.Config.GetCertificate
func (m *LetsEncryptModule) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if clientHello == nil || clientHello.ServerName == "" {
		return nil, fmt.Errorf("server name is empty")
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
			return nil, fmt.Errorf("no certificate found for domain: %s", domain)
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
		return nil, err
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
		return err
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
				RootCAs: m.rootCAs,
			},
		}
	}

	// Create client
	client, err := lego.NewClient(config)
	if err != nil {
		return err
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
func (m *LetsEncryptModule) refreshCertificates() error {
	// Request certificates for domains
	request := certificate.ObtainRequest{
		Domains: m.config.Domains,
		Bundle:  true,
	}

	certificates, err := m.client.Certificate.Obtain(request)
	if err != nil {
		return err
	}

	// Parse and store the certificates
	m.certMutex.Lock()
	defer m.certMutex.Unlock()

	for _, domain := range m.config.Domains {
		cert, err := tls.X509KeyPair(certificates.Certificate, certificates.PrivateKey)
		if err != nil {
			return fmt.Errorf("failed to parse certificate for %s: %w", domain, err)
		}
		m.certificates[domain] = &cert
	}

	return nil
}

// startRenewalTimer starts a background timer to check and renew certificates
func (m *LetsEncryptModule) startRenewalTimer() {
	// Check certificates daily
	m.renewalTicker = time.NewTicker(24 * time.Hour)

	go func() {
		for {
			select {
			case <-m.renewalTicker.C:
				// Check if certificates need renewal
				m.checkAndRenewCertificates()
			case <-m.shutdownChan:
				return
			}
		}
	}()
}

// checkAndRenewCertificates checks if certificates need renewal and renews them
func (m *LetsEncryptModule) checkAndRenewCertificates() {
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
			if err := m.renewCertificateForDomain(domain); err != nil {
				fmt.Printf("Failed to renew certificate for %s: %v\n", domain, err)
			} else {
				fmt.Printf("Successfully renewed certificate for %s\n", domain)
			}
		}
	}
}

// renewCertificateForDomain renews the certificate for a specific domain
func (m *LetsEncryptModule) renewCertificateForDomain(domain string) error {
	// Request certificate for the domain
	request := certificate.ObtainRequest{
		Domains: []string{domain},
		Bundle:  true,
	}

	certificates, err := m.client.Certificate.Obtain(request)
	if err != nil {
		return err
	}

	// Parse and store the new certificate
	cert, err := tls.X509KeyPair(certificates.Certificate, certificates.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse renewed certificate for %s: %w", domain, err)
	}

	m.certMutex.Lock()
	m.certificates[domain] = &cert
	m.certMutex.Unlock()

	return nil
}

// RevokeCertificate revokes a certificate for the specified domain
func (m *LetsEncryptModule) RevokeCertificate(domain string) error {
	m.certMutex.RLock()
	cert, exists := m.certificates[domain]
	m.certMutex.RUnlock()

	if !exists || cert == nil {
		return fmt.Errorf("no certificate found for domain %s", domain)
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
		return nil, fmt.Errorf("cloudflare provider configuration is missing")
	}

	cfg := m.config.DNSProvider.Cloudflare

	// Use API Token if provided (preferred)
	if cfg.APIToken != "" {
		return cloudflare.NewDNSProviderConfig(&cloudflare.Config{
			AuthToken: cfg.APIToken,
		})
	}

	// Fall back to environment variables
	return cloudflare.NewDNSProvider()
}

// createRoute53Provider creates an AWS Route53 DNS provider from the module configuration
func (m *LetsEncryptModule) createRoute53Provider() (challenge.Provider, error) {
	if m.config.DNSProvider == nil || m.config.DNSProvider.Route53 == nil {
		return nil, fmt.Errorf("route53 provider configuration is missing")
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

	return route53.NewDNSProviderConfig(config)
}

// createDigitalOceanProvider creates a DigitalOcean DNS provider from the module configuration
func (m *LetsEncryptModule) createDigitalOceanProvider() (challenge.Provider, error) {
	if m.config.DNSProvider == nil || m.config.DNSProvider.DigitalOcean == nil {
		return nil, fmt.Errorf("digitalocean provider configuration is missing")
	}

	cfg := m.config.DNSProvider.DigitalOcean

	if cfg.AuthToken != "" {
		return digitalocean.NewDNSProviderConfig(&digitalocean.Config{
			AuthToken: cfg.AuthToken,
		})
	}

	return nil, fmt.Errorf("digitalocean auth token is required")
}

// configureDNSProvider configures the DNS provider for ACME DNS challenges
func (m *LetsEncryptModule) configureDNSProvider() error {
	if m.config.DNSProvider == nil || m.config.DNSProvider.Provider == "" {
		return fmt.Errorf("DNS provider configuration is missing or invalid")
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
		return fmt.Errorf("unsupported DNS provider: %s", m.config.DNSProvider.Provider)
	}
}

// configureCloudflare configures the Cloudflare DNS provider
func (m *LetsEncryptModule) configureCloudflare() error {
	provider, err := m.createCloudflareProvider()
	if err != nil {
		return err
	}
	m.client.Challenge.SetDNS01Provider(provider)
	return nil
}

// configureRoute53 configures the Route53 DNS provider
func (m *LetsEncryptModule) configureRoute53() error {
	provider, err := m.createRoute53Provider()
	if err != nil {
		return err
	}
	m.client.Challenge.SetDNS01Provider(provider)
	return nil
}

// configureDigitalOcean configures the DigitalOcean DNS provider
func (m *LetsEncryptModule) configureDigitalOcean() error {
	provider, err := m.createDigitalOceanProvider()
	if err != nil {
		return err
	}
	m.client.Challenge.SetDNS01Provider(provider)
	return nil
}

// configureGoogleCloudDNS configures the Google Cloud DNS provider
func (m *LetsEncryptModule) configureGoogleCloudDNS() error {
	project := m.config.DNSConfig["project_id"]
	if project == "" {
		return fmt.Errorf("Google Cloud DNS project ID is required")
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

	m.client.Challenge.SetDNS01Provider(provider)
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
		return fmt.Errorf("Azure DNS provider requires client_id, client_secret, subscription_id, tenant_id, and resource_group")
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

	m.client.Challenge.SetDNS01Provider(provider)
	return nil
}

// configureNamecheap configures the Namecheap DNS provider
func (m *LetsEncryptModule) configureNamecheap() error {
	apiUser := m.config.DNSConfig["api_user"]
	apiKey := m.config.DNSConfig["api_key"]
	username := m.config.DNSConfig["username"]

	if apiUser == "" || apiKey == "" || username == "" {
		return fmt.Errorf("Namecheap DNS provider requires api_user, api_key, and username")
	}

	// Use environment variables as that's the most reliable way across lego versions
	os.Setenv("NAMECHEAP_API_USER", apiUser)
	os.Setenv("NAMECHEAP_API_KEY", apiKey)
	os.Setenv("NAMECHEAP_USERNAME", username)

	// Set sandbox mode if specified
	if m.config.DNSConfig["sandbox"] == "true" {
		os.Setenv("NAMECHEAP_SANDBOX", "true")
	}

	provider, err := namecheap.NewDNSProvider()
	if err != nil {
		return fmt.Errorf("failed to initialize Namecheap DNS provider: %w", err)
	}

	m.client.Challenge.SetDNS01Provider(provider)
	return nil
}

// letsEncryptHTTPProvider implements the HTTP-01 challenge provider
type letsEncryptHTTPProvider struct {
	handler http.Handler
}

// Present makes the challenge token available
func (p *letsEncryptHTTPProvider) Present(domain, token, keyAuth string) error {
	if p.handler == nil {
		return fmt.Errorf("HTTP challenge handler not configured")
	}

	// If the handler implements our custom interface, use that
	if customHandler, ok := p.handler.(ChallengeHandler); ok {
		return customHandler.PresentChallenge(domain, token, keyAuth)
	}

	// Otherwise, assume it's a standard handler that knows how to serve challenges
	return nil
}

// CleanUp removes the challenge token
func (p *letsEncryptHTTPProvider) CleanUp(domain, token, keyAuth string) error {
	if p.handler == nil {
		return nil // Nothing to clean up
	}

	// If the handler implements our custom interface, use that
	if customHandler, ok := p.handler.(ChallengeHandler); ok {
		return customHandler.CleanupChallenge(domain, token, keyAuth)
	}

	// Otherwise, assume it's a standard handler that knows how to clean up challenges
	return nil
}
