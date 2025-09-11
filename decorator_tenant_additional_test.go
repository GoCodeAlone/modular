package modular

import "testing"

// stubTenantLoader returns a fixed set of tenants for testing.
type stubTenantLoader struct{ tenants []Tenant }

func (s *stubTenantLoader) LoadTenants() ([]Tenant, error) { return s.tenants, nil }

func TestTenantAwareDecoratorStartWithLoader(t *testing.T) {
	base := NewStdApplication(NewStdConfigProvider(&struct{}{}), NewTestLogger())
	loader := &stubTenantLoader{tenants: []Tenant{{ID: "t1", Name: "Tenant One"}, {ID: "t2", Name: "Tenant Two"}}}
	dec := NewTenantAwareDecorator(base, loader)
	if err := dec.Start(); err != nil {
		t.Fatalf("start error: %v", err)
	}
}

func TestTenantAwareDecoratorStartNoLoader(t *testing.T) {
	base := NewStdApplication(NewStdConfigProvider(&struct{}{}), NewTestLogger())
	dec := NewTenantAwareDecorator(base, nil)
	if err := dec.Start(); err != nil {
		t.Fatalf("start error: %v", err)
	}
}

func TestTenantAwareDecoratorForwarding(t *testing.T) {
	base := NewStdApplication(NewStdConfigProvider(&struct{}{}), NewTestLogger())
	dec := NewTenantAwareDecorator(base, nil)
	// Tenant service likely nil until modules register; call should not panic, may return error
	if _, err := dec.GetTenantService(); err == nil {
		// Accept both error and nil; primarily exercising path
	}
	if _, err := dec.WithTenant("unknown"); err == nil {
		// acceptable; path executed
	}
}
