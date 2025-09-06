package letsencrypt

import (
    "crypto/tls"
    "errors"
    "os"
    "path/filepath"
    "testing"
)

// Test configuration validation error paths
func TestLetsEncryptConfigValidationErrors(t *testing.T) {
    cfg := &LetsEncryptConfig{}
    if err := cfg.Validate(); err == nil {
        t.Fatalf("expected error for missing email & domains")
    }

    cfg = &LetsEncryptConfig{Email: "a@b.com"}
    if err := cfg.Validate(); err == nil || !errors.Is(err, ErrDomainsRequired) {
        t.Fatalf("expected domains required error, got %v", err)
    }

    cfg = &LetsEncryptConfig{Email: "a@b.com", Domains: []string{"example.com"}, HTTPProvider: &HTTPProviderConfig{UseBuiltIn: true}, DNSProvider: &DNSProviderConfig{Provider: "cloudflare"}}
    if err := cfg.Validate(); err == nil || !errors.Is(err, ErrConflictingProviders) {
        t.Fatalf("expected conflicting providers error, got %v", err)
    }
}

// Test GetCertificate empty ServerName handling
func TestGetCertificateEmptyServerName(t *testing.T) {
    m := &LetsEncryptModule{}
    _, err := m.GetCertificate(&tls.ClientHelloInfo{})
    if err == nil || !errors.Is(err, ErrServerNameEmpty) {
        t.Fatalf("expected ErrServerNameEmpty, got %v", err)
    }
}

// Test missing certificate and wildcard fallback behavior
func TestGetCertificateForDomainMissingAndWildcard(t *testing.T) {
    m := &LetsEncryptModule{certificates: map[string]*tls.Certificate{}}
    // First, missing certificate should error
    if _, err := m.GetCertificateForDomain("missing.example.com"); err == nil || !errors.Is(err, ErrNoCertificateFound) {
        t.Fatalf("expected ErrNoCertificateFound, got %v", err)
    }

    // Add wildcard cert and request subdomain
    wildcardCert := &tls.Certificate{}
    m.certificates = map[string]*tls.Certificate{"*.example.com": wildcardCert}
    cert, err := m.GetCertificateForDomain("api.example.com")
    if err != nil {
        t.Fatalf("expected wildcard certificate, got error %v", err)
    }
    if cert != wildcardCert {
        t.Fatalf("expected returned cert to be wildcard cert")
    }
}

// Test DNS provider missing error path in configureDNSProvider
func TestConfigureDNSProviderErrors(t *testing.T) {
    m := &LetsEncryptModule{config: &LetsEncryptConfig{DNSProvider: &DNSProviderConfig{Provider: "nonexistent"}}}
    if err := m.configureDNSProvider(); err == nil || !errors.Is(err, ErrUnsupportedDNSProvider) {
        t.Fatalf("expected unsupported provider error, got %v", err)
    }
}

// Test default storage path creation logic in Validate (ensures directories created)
func TestValidateCreatesDefaultStoragePath(t *testing.T) {
    home, err := os.UserHomeDir()
    if err != nil {
        t.Skip("cannot determine home dir in test env")
    }
    // Use a temp subdir under home to avoid polluting real ~/.letsencrypt
    tempRoot := filepath.Join(home, ".letsencrypt-test-root")
    if err := os.MkdirAll(tempRoot, 0o700); err != nil {
        t.Fatalf("failed creating temp root: %v", err)
    }
    defer os.RemoveAll(tempRoot)

    // Override StoragePath empty to trigger default path logic; we temporarily swap HOME
    oldHome := os.Getenv("HOME")
    os.Setenv("HOME", tempRoot)
    defer os.Setenv("HOME", oldHome)

    cfg := &LetsEncryptConfig{Email: "a@b.com", Domains: []string{"example.com"}}
    if err := cfg.Validate(); err != nil {
        t.Fatalf("unexpected error validating config: %v", err)
    }
    if cfg.StoragePath == "" {
        t.Fatalf("expected storage path to be set")
    }
    if _, err := os.Stat(cfg.StoragePath); err != nil {
        t.Fatalf("expected storage path to exist: %v", err)
    }
}
