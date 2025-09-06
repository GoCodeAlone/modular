package letsencrypt

import (
	"context"
	"crypto/tls"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/registration"
)

// helper to create a minimal PEM cert+key (already have createMockCertificate in module_test.go)

func TestRefreshCertificatesSuccess(t *testing.T) {
	certPEM, keyPEM := createMockCertificate(t, "example.com")
	m, err := New(&LetsEncryptConfig{Email: "a@b.com", Domains: []string{"example.com"}})
	if err != nil {
		t.Fatalf("new module: %v", err)
	}
	m.user = &User{Email: "a@b.com"}
	m.obtainCertificate = func(r certificate.ObtainRequest) (*certificate.Resource, error) {
		return &certificate.Resource{Domain: "example.com", Certificate: certPEM, PrivateKey: keyPEM}, nil
	}
	m.registerAccountFunc = func(opts registration.RegisterOptions) (*registration.Resource, error) {
		return &registration.Resource{URI: "acct"}, nil
	}
	m.setHTTP01Provider = func(p challenge.Provider) error { return nil }
	// client not required because obtainCertificate & registerAccountFunc hooks used
	if err := m.refreshCertificates(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if _, ok := m.certificates["example.com"]; !ok {
		t.Fatalf("expected certificate cached")
	}
}

func TestRefreshCertificatesFailure(t *testing.T) {
	m, _ := New(&LetsEncryptConfig{Email: "a@b.com", Domains: []string{"example.com"}})
	m.obtainCertificate = func(r certificate.ObtainRequest) (*certificate.Resource, error) {
		return nil, errors.New("obtain fail")
	}
	// no real client required; hook suffices
	err := m.refreshCertificates(context.Background())
	if err == nil {
		t.Fatalf("expected error from refresh")
	}
}

func TestRenewCertificateForDomain(t *testing.T) {
	certPEM, keyPEM := createMockCertificate(t, "renew.com")
	m, _ := New(&LetsEncryptConfig{Email: "a@b.com", Domains: []string{"renew.com"}})
	m.obtainCertificate = func(r certificate.ObtainRequest) (*certificate.Resource, error) {
		return &certificate.Resource{Domain: "renew.com", Certificate: certPEM, PrivateKey: keyPEM}, nil
	}
	// no real client required; hook suffices
	if err := m.renewCertificateForDomain(context.Background(), "renew.com"); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if _, ok := m.certificates["renew.com"]; !ok {
		t.Fatalf("expected renewed cert present")
	}
}

func TestRevokeCertificate(t *testing.T) {
	certPEM, keyPEM := createMockCertificate(t, "revoke.com")
	tlsPair, _ := tls.X509KeyPair(certPEM, keyPEM)
	m, _ := New(&LetsEncryptConfig{Email: "a@b.com", Domains: []string{"revoke.com"}})
	m.certificates["revoke.com"] = &tlsPair
	revoked := false
	m.revokeCertificate = func(raw []byte) error { revoked = true; return nil }
	if err := m.RevokeCertificate("revoke.com"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if revoked == false {
		t.Fatalf("expected revoke called")
	}
	if _, ok := m.certificates["revoke.com"]; ok {
		t.Fatalf("expected cert removed after revoke")
	}
}

// New tests to cover additional error paths in Start/init sequence
func TestStart_AccountRegistrationError(t *testing.T) {
	m, err := New(&LetsEncryptConfig{Email: "a@b.com", Domains: []string{"err.com"}})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	// inject user to bypass initUser path except registration
	m.user = &User{Email: "a@b.com"}
	// force registerAccountFunc to error
	m.registerAccountFunc = func(opts registration.RegisterOptions) (*registration.Resource, error) {
		return nil, errors.New("register boom")
	}
	// other hooks so initClient proceeds until registration
	m.setHTTP01Provider = func(p challenge.Provider) error { return nil }
	m.obtainCertificate = func(r certificate.ObtainRequest) (*certificate.Resource, error) {
		return nil, errors.New("should not reach obtain if registration fails")
	}
	if err := m.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "register boom") {
		t.Fatalf("expected register boom error, got %v", err)
	}
}

func TestStart_HTTPProviderError(t *testing.T) {
	m, err := New(&LetsEncryptConfig{Email: "a@b.com", Domains: []string{"http.com"}})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	m.user = &User{Email: "a@b.com"}
	m.registerAccountFunc = func(opts registration.RegisterOptions) (*registration.Resource, error) {
		return &registration.Resource{}, nil
	}
	m.setHTTP01Provider = func(p challenge.Provider) error { return errors.New("http provider boom") }
	if err := m.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "http provider boom") {
		t.Fatalf("expected http provider boom, got %v", err)
	}
}

func TestStart_DNSProviderUnsupported(t *testing.T) {
	cfg := &LetsEncryptConfig{Email: "a@b.com", Domains: []string{"dns.com"}, DNSProvider: &DNSProviderConfig{Provider: "unsupported"}, UseDNS: true}
	m, err := New(cfg)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	m.user = &User{Email: "a@b.com"}
	m.registerAccountFunc = func(opts registration.RegisterOptions) (*registration.Resource, error) {
		return &registration.Resource{}, nil
	}
	m.setDNS01Provider = func(p challenge.Provider) error { return nil }
	if err := m.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported provider error, got %v", err)
	}
}

func TestRefreshCertificates_ObtainError(t *testing.T) {
	m, _ := New(&LetsEncryptConfig{Email: "a@b.com", Domains: []string{"example.com"}})
	// create user via initUser to ensure private key present
	u, err := m.initUser()
	if err != nil {
		t.Fatalf("initUser: %v", err)
	}
	m.user = u
	m.obtainCertificate = func(r certificate.ObtainRequest) (*certificate.Resource, error) {
		return nil, errors.New("obtain boom")
	}
	m.registerAccountFunc = func(opts registration.RegisterOptions) (*registration.Resource, error) {
		return &registration.Resource{}, nil
	}
	m.setHTTP01Provider = func(p challenge.Provider) error { return nil }
	if err := m.initClient(); err != nil {
		t.Fatalf("initClient: %v", err)
	}
	if err := m.refreshCertificates(context.Background()); err == nil || !strings.Contains(err.Error(), "obtain boom") {
		t.Fatalf("expected obtain boom error, got %v", err)
	}
}

// Silence unused warnings for helper types/vars
var _ = time.Second
