package modular

import "testing"

// simpleTenantLoader used to exercise tenant aware decorator wiring in builder tests.
type simpleTenantLoader struct{}

func (s *simpleTenantLoader) LoadTenants() ([]Tenant, error) { return []Tenant{}, nil }

// TestBuilderWithBaseApplication ensures WithBaseApplication bypasses default creation paths.
func TestBuilderWithBaseApplication(t *testing.T) {
	base := NewStdApplication(NewStdConfigProvider(&struct{}{}), NewTestLogger())
	app, err := NewApplication(WithBaseApplication(base), WithModules())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app != base {
		t.Fatalf("expected returned app to be provided base instance")
	}
}

// TestTenantAwareConfigDecorator ensures the decorator returned by helper is applied via builder config decorators chain.
// NOTE: TenantAwareConfigDecorator already tested in config_decorators_test.go. No duplication here.
