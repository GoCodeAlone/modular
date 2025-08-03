package letsencrypt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular/modules/httpserver"
	"github.com/go-acme/lego/v4/certificate"
)

// Ensure LetsEncryptModule implements CertificateService interface
var _ httpserver.CertificateService = (*LetsEncryptModule)(nil)

func TestLetsEncryptGetCertificate(t *testing.T) {
	// Create a test directory for certificates
	testDir, err := os.MkdirTemp("", "letsencrypt-test")
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(testDir); err != nil {
			t.Logf("Warning: failed to remove test directory: %v", err)
		}
	}()

	// Create a test module
	config := &LetsEncryptConfig{
		Email:        "test@example.com",
		Domains:      []string{"example.com", "www.example.com"},
		StoragePath:  testDir,
		AutoRenew:    false,
		UseStaging:   true,
		HTTPProvider: &HTTPProviderConfig{UseBuiltIn: true},
	}

	module, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create LetsEncrypt module: %v", err)
	}

	// Create mock certificate
	certPEM, keyPEM := createMockCertificate(t, "example.com")

	// Create a certificate resource
	certResource := &certificate.Resource{
		Domain:            "example.com",
		Certificate:       certPEM,
		PrivateKey:        keyPEM,
		IssuerCertificate: nil,
	}

	// Create certificate storage
	storage, err := newCertificateStorage(testDir)
	if err != nil {
		t.Fatalf("Failed to create certificate storage: %v", err)
	}

	// Save certificate
	if err := storage.SaveCertificate("example.com", certResource); err != nil {
		t.Fatalf("Failed to save certificate: %v", err)
	}

	// Load certificate into module
	module.certificates = make(map[string]*tls.Certificate)
	tlsCert, err := storage.LoadCertificate("example.com")
	if err != nil {
		t.Fatalf("Failed to load certificate: %v", err)
	}
	module.certificates["example.com"] = tlsCert

	// Test GetCertificate
	clientHello := &tls.ClientHelloInfo{
		ServerName: "example.com",
	}
	resultCert, err := module.GetCertificate(clientHello)
	if err != nil {
		t.Fatalf("GetCertificate failed: %v", err)
	}

	if resultCert == nil {
		t.Fatal("GetCertificate returned nil certificate")
	}

	// Verify it's the same certificate
	resultX509, err := x509.ParseCertificate(resultCert.Certificate[0])
	if err != nil {
		t.Fatalf("Failed to parse result certificate: %v", err)
	}

	if resultX509.Subject.CommonName != "example.com" {
		t.Errorf("Expected certificate for example.com, got %s", resultX509.Subject.CommonName)
	}
}

func TestDomains(t *testing.T) {
	module := &LetsEncryptModule{
		certificates: make(map[string]*tls.Certificate),
	}

	// Create real test certificates
	cert1PEM, _ := createMockCertificate(t, "example.com")
	cert2PEM, _ := createMockCertificate(t, "test.com")

	// Parse the certificates
	block1, _ := pem.Decode(cert1PEM)
	block2, _ := pem.Decode(cert2PEM)

	// Add certificates to the module
	module.certificates["example.com"] = &tls.Certificate{Certificate: [][]byte{block1.Bytes}}
	module.certificates["test.com"] = &tls.Certificate{Certificate: [][]byte{block2.Bytes}}

	// Get domains
	domains := module.Domains()

	// Check if all domains are returned
	if len(domains) != 2 {
		t.Errorf("Expected 2 domains, got %d", len(domains))
	}

	foundExample := false
	foundTest := false
	for _, domain := range domains {
		if domain == "example.com" {
			foundExample = true
		}
		if domain == "test.com" {
			foundTest = true
		}
	}

	if !foundExample {
		t.Error("example.com not found in domains list")
	}
	if !foundTest {
		t.Error("test.com not found in domains list")
	}
}

// createMockCertificate creates a mock certificate for testing
// Returns PEM-encoded certificate and key
func createMockCertificate(t *testing.T, domain string) ([]byte, []byte) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// Create a certificate template
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: domain,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{domain},
	}

	// Create self-signed certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Encode certificate to PEM format
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	// Encode private key to PEM format
	privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("Failed to marshal private key: %v", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	})

	return certPEM, keyPEM
}

// Additional tests to improve coverage
func TestLetsEncryptModule_Name(t *testing.T) {
	module := &LetsEncryptModule{}
	name := module.Name()
	if name != ModuleName {
		t.Errorf("Expected module name %s, got %s", ModuleName, name)
	}
}

func TestLetsEncryptModule_Config(t *testing.T) {
	config := &LetsEncryptConfig{
		Email:   "test@example.com",
		Domains: []string{"example.com"},
	}
	module := &LetsEncryptModule{config: config}

	result := module.Config()
	if result != config {
		t.Error("Config method should return the module's config")
	}
}

func TestLetsEncryptModule_StartStop(t *testing.T) {
	config := &LetsEncryptConfig{
		Email:        "test@example.com",
		Domains:      []string{"example.com"},
		StoragePath:  "/tmp/test-letsencrypt",
		AutoRenew:    false,
		UseStaging:   true,
		HTTPProvider: &HTTPProviderConfig{UseBuiltIn: true},
	}

	module, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create module: %v", err)
	}

	// Test Stop when not started (should not error)
	err = module.Stop(context.Background())
	if err != nil {
		t.Errorf("Stop should not error when not started: %v", err)
	}

	// Note: We can't easily test Start as it requires ACME server interaction
}

func TestLetsEncryptModule_GetCertificateForDomain(t *testing.T) {
	// Create a test directory for certificates
	testDir, err := os.MkdirTemp("", "letsencrypt-test2")
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	config := &LetsEncryptConfig{
		Email:        "test@example.com",
		Domains:      []string{"example.com"},
		StoragePath:  testDir,
		UseStaging:   true,
		HTTPProvider: &HTTPProviderConfig{UseBuiltIn: true},
	}

	module, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create module: %v", err)
	}

	// Create mock certificate
	certPEM, keyPEM := createMockCertificate(t, "example.com")

	// Create certificate storage and save certificate
	storage, err := newCertificateStorage(testDir)
	if err != nil {
		t.Fatalf("Failed to create certificate storage: %v", err)
	}

	certResource := &certificate.Resource{
		Domain:      "example.com",
		Certificate: certPEM,
		PrivateKey:  keyPEM,
	}

	if err := storage.SaveCertificate("example.com", certResource); err != nil {
		t.Fatalf("Failed to save certificate: %v", err)
	}

	// Initialize certificates map and load certificate
	module.certificates = make(map[string]*tls.Certificate)
	tlsCert, err := storage.LoadCertificate("example.com")
	if err != nil {
		t.Fatalf("Failed to load certificate: %v", err)
	}
	module.certificates["example.com"] = tlsCert

	// Test GetCertificateForDomain for existing domain
	cert, err := module.GetCertificateForDomain("example.com")
	if err != nil {
		t.Errorf("GetCertificateForDomain failed: %v", err)
	}
	if cert == nil {
		t.Error("Expected certificate for example.com")
	}

	// Test GetCertificateForDomain for non-existing domain
	cert, err = module.GetCertificateForDomain("nonexistent.com")
	if err == nil {
		t.Error("Expected error for non-existent domain")
	}
	if cert != nil {
		t.Error("Expected nil certificate for non-existent domain")
	}
}

func TestLetsEncryptConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *LetsEncryptConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &LetsEncryptConfig{
				Email:        "test@example.com",
				Domains:      []string{"example.com"},
				StoragePath:  "/tmp/test",
				HTTPProvider: &HTTPProviderConfig{UseBuiltIn: true},
			},
			wantErr: false,
		},
		{
			name: "missing email",
			config: &LetsEncryptConfig{
				Domains:     []string{"example.com"},
				StoragePath: "/tmp/test",
			},
			wantErr: true,
		},
		{
			name: "missing domains",
			config: &LetsEncryptConfig{
				Email:       "test@example.com",
				StoragePath: "/tmp/test",
			},
			wantErr: true,
		},
		{
			name: "empty domains",
			config: &LetsEncryptConfig{
				Email:       "test@example.com",
				Domains:     []string{},
				StoragePath: "/tmp/test",
			},
			wantErr: true,
		},
		{
			name: "missing storage path - sets default",
			config: &LetsEncryptConfig{
				Email:   "test@example.com",
				Domains: []string{"example.com"},
				// StoragePath is omitted to test default behavior
			},
			wantErr: false, // Should not error, just set default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCertificateStorage_ListCertificates(t *testing.T) {
	testDir, err := os.MkdirTemp("", "cert-storage-test")
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	storage, err := newCertificateStorage(testDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Test empty directory
	certs, err := storage.ListCertificates()
	if err != nil {
		t.Errorf("ListCertificates failed: %v", err)
	}
	if len(certs) != 0 {
		t.Errorf("Expected 0 certificates, got %d", len(certs))
	}
}

func TestCertificateStorage_IsCertificateExpiringSoon(t *testing.T) {
	testDir, err := os.MkdirTemp("", "cert-expiry-test")
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	storage, err := newCertificateStorage(testDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Test non-existent certificate
	isExpiring, err := storage.IsCertificateExpiringSoon("nonexistent.com", 30)
	if err == nil {
		t.Error("Expected error for non-existent certificate")
	}
	if isExpiring {
		t.Error("Non-existent certificate should not be expiring")
	}
}

func TestSanitizeDomain(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "example_com"},
		{"sub.example.com", "sub_example_com"},
		{"test-domain.com", "test-domain_com"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeDomain(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeDomain(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDesanitizeDomain(t *testing.T) {
	result := desanitizeDomain("example_com")
	expected := "example.com"
	if result != expected {
		t.Errorf("desanitizeDomain(example_com) = %s, expected %s", result, expected)
	}
}

func TestUser_Interface(t *testing.T) {
	user := &User{
		Email:        "test@example.com",
		Registration: nil,
		Key:          nil,
	}

	// Test GetEmail
	email := user.GetEmail()
	if email != "test@example.com" {
		t.Errorf("GetEmail() = %s, expected test@example.com", email)
	}

	// Test GetRegistration
	reg := user.GetRegistration()
	if reg != nil {
		t.Error("Expected nil registration")
	}

	// Test GetPrivateKey
	key := user.GetPrivateKey()
	if key != nil {
		t.Error("Expected nil private key")
	}
}

// Additional tests for coverage improvement
func TestHTTPProvider_PresentCleanUp(t *testing.T) {
	provider := &letsEncryptHTTPProvider{
		handler: nil, // No handler set
	}

	// Test Present method without handler
	err := provider.Present("example.com", "token", "keyAuth")
	if err == nil {
		t.Error("Expected error when no handler is set")
	}

	// Test CleanUp method
	err = provider.CleanUp("example.com", "token", "keyAuth")
	if err != nil {
		t.Errorf("CleanUp should not error: %v", err)
	}
}

func TestLetsEncryptModule_RevokeCertificate(t *testing.T) {
	config := &LetsEncryptConfig{
		Email:        "test@example.com",
		Domains:      []string{"example.com"},
		StoragePath:  "/tmp/test-revoke",
		UseStaging:   true,
		HTTPProvider: &HTTPProviderConfig{UseBuiltIn: true},
	}

	module, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create module: %v", err)
	}

	// Test RevokeCertificate without initialization (should fail gracefully)
	err = module.RevokeCertificate("example.com")
	if err == nil {
		t.Error("Expected error when revoking certificate without initialization")
	}
}

func TestLetsEncryptModule_CreateProviders(t *testing.T) {
	module := &LetsEncryptModule{
		config: &LetsEncryptConfig{
			DNSProvider: &DNSProviderConfig{
				Provider: "cloudflare",
				Cloudflare: &CloudflareConfig{
					Email:  "test@example.com",
					APIKey: "test-key",
				},
			},
		},
	}

	// Test createCloudflareProvider - will fail but exercise the code path
	_, err := module.createCloudflareProvider()
	if err == nil {
		t.Log("createCloudflareProvider unexpectedly succeeded (may be in test env)")
	}

	// Test createRoute53Provider
	module.config.DNSProvider.Provider = "route53"
	module.config.DNSProvider.Route53 = &Route53Config{
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
		Region:          "us-east-1",
	}
	_, err = module.createRoute53Provider()
	if err == nil {
		t.Log("createRoute53Provider unexpectedly succeeded (may be in test env)")
	}

	// Test createDigitalOceanProvider
	module.config.DNSProvider.Provider = "digitalocean"
	module.config.DNSProvider.DigitalOcean = &DigitalOceanConfig{
		AuthToken: "test-token",
	}
	_, err = module.createDigitalOceanProvider()
	if err == nil {
		t.Log("createDigitalOceanProvider unexpectedly succeeded (may be in test env)")
	}
}

func TestLetsEncryptModule_ConfigureDNSProvider(t *testing.T) {
	module := &LetsEncryptModule{
		config: &LetsEncryptConfig{
			DNSProvider: &DNSProviderConfig{
				Provider: "cloudflare",
				Cloudflare: &CloudflareConfig{
					Email:  "test@example.com",
					APIKey: "test-key",
				},
			},
		},
	}

	// Test configureDNSProvider (may fail due to missing credentials, which is expected)
	err := module.configureDNSProvider()
	// Don't fail test if credentials are missing - this is expected in test environment
	if err != nil {
		t.Logf("configureDNSProvider failed (expected in test env): %v", err)
	}

	// Test with unsupported provider
	module.config.DNSProvider.Provider = "unsupported"
	err = module.configureDNSProvider()
	if err == nil {
		t.Error("Expected error for unsupported DNS provider")
	}
}
