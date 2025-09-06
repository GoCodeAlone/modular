package letsencrypt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestUserAccessors covers simple accessor methods GetEmail, GetRegistration, GetPrivateKey
func TestUserAccessors(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("key gen: %v", err)
	}
	u := &User{Email: "test@example.com", Key: key}
	if u.GetEmail() != "test@example.com" {
		t.Fatalf("expected email accessor to return value")
	}
	if u.GetPrivateKey() == nil {
		t.Fatalf("expected private key")
	}
	if u.GetRegistration() != nil {
		t.Fatalf("expected nil registration by default")
	}
}

// TestSanitizeRoundTrip ensures sanitizeDomain/desanitizeDomain are symmetric
func TestSanitizeRoundTrip(t *testing.T) {
	in := "sub.domain.example"
	if got := desanitizeDomain(sanitizeDomain(in)); got != in {
		t.Fatalf("round trip mismatch: %s != %s", got, in)
	}
}

// TestListCertificatesEmpty ensures empty directory returns empty slice
func TestListCertificatesEmpty(t *testing.T) {
	dir := t.TempDir()
	store, err := newCertificateStorage(dir)
	if err != nil {
		t.Fatalf("storage init: %v", err)
	}
	domains, err := store.ListCertificates()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(domains) != 0 {
		t.Fatalf("expected 0 domains, got %d", len(domains))
	}
}

// TestIsCertificateExpiringSoon creates a short lived cert and checks expiring logic
func TestIsCertificateExpiringSoon(t *testing.T) {
	dir := t.TempDir()
	store, err := newCertificateStorage(dir)
	if err != nil {
		t.Fatalf("storage init: %v", err)
	}

	// Create directory structure and a fake cert with NotAfter in 1 day
	domain := "example.com"
	path := filepath.Join(dir, sanitizeDomain(domain))
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Generate a self-signed cert with 24h validity
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	tmpl := x509.Certificate{SerialNumber: newSerial(t), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(filepath.Join(path, "cert.pem"), pemBytes, 0600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "key.pem"), pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}), 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	soon, err := store.IsCertificateExpiringSoon(domain, 2) // threshold 2 days; cert expires in 1
	if err != nil {
		t.Fatalf("expiring soon: %v", err)
	}
	if !soon {
		t.Fatalf("expected cert to be considered expiring soon")
	}

	later, err := store.IsCertificateExpiringSoon(domain, 0) // threshold 0 days; not yet expired
	if err != nil {
		t.Fatalf("expiring check: %v", err)
	}
	if later {
		t.Fatalf("did not expect cert to be expiring with 0 day threshold")
	}
}

func newSerial(t *testing.T) *big.Int {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("serial: %v", err)
	}
	return new(big.Int).SetBytes(b)
}
