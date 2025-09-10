package modular

import (
	"sync"
	"testing"
)

type sampleCfg struct{ Value string }

func TestTenantConfigProvider_ErrorPathsAndDefaults(t *testing.T) {
	tcp := NewTenantConfigProvider(NewStdConfigProvider(&sampleCfg{Value: "default"}))

	// Getting non-existent tenant
	if _, err := tcp.GetTenantConfig(TenantID("missing"), "section"); err == nil {
		t.Fatalf("expected error for missing tenant")
	}

	// Initialize tenant via SetTenantConfig with nil provider (should be ignored and not create section)
	tcp.SetTenantConfig(TenantID("t1"), "S1", nil)
	if tcp.HasTenantConfig(TenantID("t1"), "S1") {
		t.Fatalf("nil provider should not create config")
	}

	// Provider with nil underlying config ignored
	nilProvider := NewStdConfigProvider(nil)
	tcp.SetTenantConfig(TenantID("t1"), "NilConfig", nilProvider)
	if tcp.HasTenantConfig(TenantID("t1"), "NilConfig") {
		t.Fatalf("provider with nil config should not register")
	}

	// Valid provider
	cfgProv := NewStdConfigProvider(&sampleCfg{Value: "v1"})
	tcp.SetTenantConfig(TenantID("t1"), "SectionA", cfgProv)
	if !tcp.HasTenantConfig(TenantID("t1"), "SectionA") {
		t.Fatalf("expected SectionA present")
	}

	// Retrieve existing
	got, err := tcp.GetTenantConfig(TenantID("t1"), "SectionA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetConfig().(*sampleCfg).Value != "v1" {
		t.Fatalf("unexpected config value")
	}

	// Missing section error
	if _, err := tcp.GetTenantConfig(TenantID("t1"), "Missing"); err == nil {
		t.Fatalf("expected error for missing section")
	}
}

func TestTenantConfigProvider_ConcurrentSetAndGet(t *testing.T) {
	tcp := NewTenantConfigProvider(nil)
	tenant := TenantID("concurrent")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tcp.SetTenantConfig(tenant, "S", NewStdConfigProvider(&sampleCfg{Value: "v"}))
			_ = tcp.HasTenantConfig(tenant, "S")
		}(i)
	}
	wg.Wait()

	if !tcp.HasTenantConfig(tenant, "S") {
		t.Fatalf("expected config after concurrent sets")
	}
	if _, err := tcp.GetTenantConfig(tenant, "S"); err != nil {
		t.Fatalf("expected retrieval success: %v", err)
	}
}
