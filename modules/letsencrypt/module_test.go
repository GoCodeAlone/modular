package letsencrypt

import (
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

	"github.com/CrisisTextLine/modular/modules/httpserver"
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
