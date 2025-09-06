package letsencrypt

import (
	"context"
	"crypto/tls"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/registration"
)

// Cloudflare: missing config struct
func TestCreateCloudflareProviderMissing(t *testing.T) {
	m, _ := New(&LetsEncryptConfig{Email: "a@b.com", Domains: []string{"c.com"}, UseDNS: true, DNSProvider: &DNSProviderConfig{Provider: "cloudflare"}})
	u, err := m.initUser()
	if err != nil {
		t.Fatalf("initUser: %v", err)
	}
	m.user = u
	m.registerAccountFunc = func(opts registration.RegisterOptions) (*registration.Resource, error) {
		return &registration.Resource{}, nil
	}
	err = m.initClient()
	if err == nil || !strings.Contains(err.Error(), "cloudflare") {
		t.Fatalf("expected cloudflare error, got %v", err)
	}
}

// DigitalOcean: missing token
func TestCreateDigitalOceanProviderMissingToken(t *testing.T) {
	// call createDigitalOceanProvider directly
	m := &LetsEncryptModule{config: &LetsEncryptConfig{DNSProvider: &DNSProviderConfig{Provider: "digitalocean", DigitalOcean: &DigitalOceanConfig{}}}}
	if _, err := m.createDigitalOceanProvider(); err == nil || err.Error() != ErrDigitalOceanTokenRequired.Error() {
		t.Fatalf("expected digitalocean token required error, got %v", err)
	}
}

// Route53: partial creds should still succeed provider creation with missing optional fields
func TestCreateRoute53ProviderPartialCreds(t *testing.T) {
	m := &LetsEncryptModule{config: &LetsEncryptConfig{DNSProvider: &DNSProviderConfig{Provider: "route53", Route53: &Route53Config{AccessKeyID: "id", SecretAccessKey: "secret"}}}, user: &User{Email: "x@y.z"}}
	// Need client to set provider later, but here we only test createRoute53Provider logic indirectly via configureRoute53? Simpler: just call createRoute53Provider (needs config.Route53 present)
	if _, err := m.createRoute53Provider(); err != nil {
		t.Fatalf("unexpected error creating partial route53 provider: %v", err)
	}
}

// Azure: incomplete config
func TestConfigureAzureDNSIncomplete(t *testing.T) {
	m := &LetsEncryptModule{config: &LetsEncryptConfig{DNSConfig: map[string]string{"client_id": "id"}}}
	if err := m.configureAzureDNS(); err == nil || err != ErrAzureDNSConfigIncomplete {
		t.Fatalf("expected incomplete azure config error, got %v", err)
	}
}

// Namecheap: incomplete config
func TestConfigureNamecheapIncomplete(t *testing.T) {
	m := &LetsEncryptModule{config: &LetsEncryptConfig{DNSConfig: map[string]string{"api_user": "u"}}}
	if err := m.configureNamecheap(); err == nil || err != ErrNamecheapConfigIncomplete {
		t.Fatalf("expected incomplete namecheap config error, got %v", err)
	}
}

// Google Cloud: missing project id
func TestConfigureGoogleCloudMissingProject(t *testing.T) {
	m := &LetsEncryptModule{config: &LetsEncryptConfig{DNSConfig: map[string]string{}}}
	if err := m.configureGoogleCloudDNS(); err == nil || err != ErrGoogleCloudProjectRequired {
		t.Fatalf("expected missing project error, got %v", err)
	}
}

// Renewal timer coverage using injected short interval
func TestStartRenewalTimerIntervalHook(t *testing.T) {
	certPEM, keyPEM := createMockCertificate(t, "short.com")
	m, _ := New(&LetsEncryptConfig{Email: "a@b.com", Domains: []string{"short.com"}, AutoRenew: true, RenewBeforeDays: 30})
	// prepare pre-existing cert expiring soon to trigger renewal path rapidly
	pair, _ := tls.X509KeyPair(certPEM, keyPEM)
	m.certificates["short.com"] = &pair
	m.user, _ = m.initUser()
	var renewed int32
	m.obtainCertificate = func(r certificate.ObtainRequest) (*certificate.Resource, error) {
		atomic.StoreInt32(&renewed, 1)
		return &certificate.Resource{Certificate: certPEM, PrivateKey: keyPEM}, nil
	}
	m.registerAccountFunc = func(opts registration.RegisterOptions) (*registration.Resource, error) {
		return &registration.Resource{}, nil
	}
	m.setHTTP01Provider = func(p challenge.Provider) error { return nil }
	m.renewalInterval = func() time.Duration { return 10 * time.Millisecond }
	if err := m.initClient(); err != nil {
		t.Fatalf("initClient: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.startRenewalTimer(ctx)
	time.Sleep(30 * time.Millisecond)
	if atomic.LoadInt32(&renewed) != 1 {
		t.Fatalf("expected renewal to occur with short interval")
	}
	close(m.shutdownChan)
}
