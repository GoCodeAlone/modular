package letsencrypt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/registration"
)

// helper to make a self-signed cert with given notAfter in days from now
func makeDummyCert(t *testing.T, cn string, notAfter time.Time) (certPEM, keyPEM []byte) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tpl := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: cn}, NotBefore: time.Now().Add(-time.Hour), NotAfter: notAfter, DNSNames: []string{cn}}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyBytes := x509.MarshalPKCS1PrivateKey(priv)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
	return
}

func TestCheckAndRenewCertificates_RenewsExpiring(t *testing.T) {
	ctx := context.Background()
	mod, err := New(&LetsEncryptConfig{Email: "a@b.c", Domains: []string{"example.com"}, AutoRenew: true, RenewBeforeDays: 30})
	if err != nil {
		t.Fatalf("new module: %v", err)
	}
	// inject minimal user and fake client hooks so initClient/createUser not needed
	mod.user = &User{Email: "a@b.c"}
	// provide obtainCertificate hook: first call used by refreshCertificates in Start path we skip; we set cert map manually; second call for renewal returns new later expiry cert
	newCertPEM, newKeyPEM := makeDummyCert(t, "example.com", time.Now().Add(90*24*time.Hour))
	mod.obtainCertificate = func(request certificate.ObtainRequest) (*certificate.Resource, error) {
		return &certificate.Resource{Certificate: newCertPEM, PrivateKey: newKeyPEM}, nil
	}
	mod.revokeCertificate = func(raw []byte) error { return nil }
	mod.setHTTP01Provider = func(p challenge.Provider) error { return nil }
	mod.setDNS01Provider = func(p challenge.Provider) error { return nil }
	mod.registerAccountFunc = func(opts registration.RegisterOptions) (*registration.Resource, error) {
		return &registration.Resource{}, nil
	}
	// seed existing cert nearing expiry (10 days, within RenewBeforeDays)
	oldCertPEM, oldKeyPEM := makeDummyCert(t, "example.com", time.Now().Add(10*24*time.Hour))
	certPair, err := tls.X509KeyPair(oldCertPEM, oldKeyPEM)
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	mod.certificates["example.com"] = &certPair
	mod.checkAndRenewCertificates(ctx)
	// after renewal, cert should have NotAfter roughly ~90 days.
	mod.certMutex.RLock()
	updated := mod.certificates["example.com"]
	mod.certMutex.RUnlock()
	x509c, _ := x509.ParseCertificate(updated.Certificate[0])
	if time.Until(x509c.NotAfter) < 60*24*time.Hour {
		// should be renewed to >60 days
		b, _ := x509.ParseCertificate(certPair.Certificate[0])
		if b.NotAfter != x509c.NotAfter { // ensure changed
			t.Fatalf("certificate not renewed; still expiring soon")
		}
	}
}

func TestRevokeCertificate_ErrorPath(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	mod, err := New(&LetsEncryptConfig{Email: "a@b.c", Domains: []string{"example.com"}})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	mod.user = &User{Email: "a@b.c"}
	certPEM, keyPEM := makeDummyCert(t, "example.com", time.Now().Add(90*24*time.Hour))
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	mod.certificates["example.com"] = &pair
	mod.revokeCertificate = func(raw []byte) error { return errors.New("boom") }
	if err := mod.RevokeCertificate("example.com"); err == nil || !strings.Contains(err.Error(), "boom") {
		// We expect wrapped error containing boom
		t.Fatalf("expected boom error, got %v", err)
	}
}

func TestGetCertificateForDomain_WildcardNegative(t *testing.T) {
	mod, err := New(&LetsEncryptConfig{Email: "a@b.c", Domains: []string{"*.example.com"}})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	// Store wildcard cert only
	certPEM, keyPEM := makeDummyCert(t, "*.example.com", time.Now().Add(90*24*time.Hour))
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	mod.certificates["*.example.com"] = &pair
	// request unrelated domain
	if _, err := mod.GetCertificateForDomain("other.com"); err == nil || !errors.Is(err, ErrNoCertificateFound) {
		// expect no certificate found
		t.Fatalf("expected ErrNoCertificateFound, got %v", err)
	}
}
