package modular

import "testing"

// simple tenant loader for tests
type testTenantLoader struct{}

// LoadTenants returns an empty slice of Tenant to satisfy TenantLoader.
func (l *testTenantLoader) LoadTenants() ([]Tenant, error) { return []Tenant{}, nil }

func TestInstanceAwareConfigDecorator(t *testing.T) {
	cfg := &minimalConfig{Value: "base"}
	cp := NewStdConfigProvider(cfg)
	dec := &instanceAwareConfigDecorator{}
	wrapped := dec.DecorateConfig(cp)
	if wrapped.GetConfig().(*minimalConfig).Value != "base" {
		t.Fatalf("decorated config mismatch")
	}
	if dec.Name() != "InstanceAware" {
		t.Fatalf("unexpected name: %s", dec.Name())
	}
}

func TestTenantAwareConfigDecorator(t *testing.T) {
	cfg := &minimalConfig{Value: "base"}
	cp := NewStdConfigProvider(cfg)
	dec := &tenantAwareConfigDecorator{loader: &testTenantLoader{}}
	wrapped := dec.DecorateConfig(cp)
	if wrapped.GetConfig().(*minimalConfig).Value != "base" {
		t.Fatalf("decorated config mismatch")
	}
	if dec.Name() != "TenantAware" {
		t.Fatalf("unexpected name: %s", dec.Name())
	}

	tenantCfg, err := wrapped.(*tenantAwareConfigProvider).GetTenantConfig("t1")
	if err != nil || tenantCfg.(*minimalConfig).Value != "base" {
		t.Fatalf("GetTenantConfig unexpected result: %v", err)
	}

	// error path (nil loader)
	decNil := &tenantAwareConfigDecorator{}
	wrappedNil := decNil.DecorateConfig(cp)
	_, err = wrappedNil.(*tenantAwareConfigProvider).GetTenantConfig("t1")
	if err == nil {
		t.Fatalf("expected error when loader nil")
	}
}
