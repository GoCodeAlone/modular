package modular

import "testing"

func TestTenantConfigProvider_New(t *testing.T) {
	defaultCfg := &minimalConfig{Value: "default"}
	tcp := NewTenantConfigProvider(NewStdConfigProvider(defaultCfg))

	// missing tenant
	if _, err := tcp.GetTenantConfig("nope", "sec"); err == nil {
		t.Fatalf("expected tenant not found error")
	}

	// set invalid (nil provider) should be ignored
	tcp.SetTenantConfig("t1", "sec", nil)
	if tcp.HasTenantConfig("t1", "sec") {
		t.Fatalf("should not have config")
	}

	// valid provider
	cfg := &minimalConfig{Value: "tenant"}
	tcp.SetTenantConfig("t1", "app", NewStdConfigProvider(cfg))
	if !tcp.HasTenantConfig("t1", "app") {
		t.Fatalf("expected config present")
	}
	got, err := tcp.GetTenantConfig("t1", "app")
	if err != nil || got.GetConfig().(*minimalConfig).Value != "tenant" {
		t.Fatalf("unexpected tenant config: %v", err)
	}

	// missing section
	if _, err := tcp.GetTenantConfig("t1", "missing"); err == nil {
		t.Fatalf("expected missing section error")
	}
}
