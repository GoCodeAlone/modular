package modular

import (
	"context"
	"testing"
)

func TestNewTenantContext(t *testing.T) {
	// Test case: Create a tenant context with a valid tenant ID
	ctx := context.Background()
	tenantID := TenantID("tenant123")

	tenantCtx := NewTenantContext(ctx, tenantID)

	if tenantCtx == nil {
		t.Fatal("Expected non-nil TenantContext, got nil")
	}

	if tenantCtx.Context != ctx {
		t.Errorf("Expected underlying context to be the same as provided context")
	}

	if tenantCtx.tenantID != tenantID {
		t.Errorf("Expected tenant ID %s, got %s", tenantID, tenantCtx.tenantID)
	}
}

func TestTenantContext_GetTenantID(t *testing.T) {
	// Test case: Get tenant ID from a tenant context
	ctx := context.Background()
	tenantID := TenantID("tenant456")

	tenantCtx := NewTenantContext(ctx, tenantID)
	retrievedID := tenantCtx.GetTenantID()

	if retrievedID != tenantID {
		t.Errorf("Expected tenant ID %s, got %s", tenantID, retrievedID)
	}

	// Test case: Empty tenant ID
	emptyTenantID := TenantID("")
	emptyTenantCtx := NewTenantContext(ctx, emptyTenantID)
	retrievedEmptyID := emptyTenantCtx.GetTenantID()

	if retrievedEmptyID != emptyTenantID {
		t.Errorf("Expected empty tenant ID, got %s", retrievedEmptyID)
	}
}

func TestGetTenantIDFromContext(t *testing.T) {
	// Test case: Extract tenant ID from a tenant context
	ctx := context.Background()
	tenantID := TenantID("tenant789")

	tenantCtx := NewTenantContext(ctx, tenantID)
	retrievedID, ok := GetTenantIDFromContext(tenantCtx)

	if !ok {
		t.Error("Expected ok to be true when extracting tenant ID from a TenantContext")
	}

	if retrievedID != tenantID {
		t.Errorf("Expected tenant ID %s, got %s", tenantID, retrievedID)
	}

	// Test case: Attempt to extract tenant ID from a non-tenant context
	regularCtx := context.Background()
	_, ok = GetTenantIDFromContext(regularCtx)

	if ok {
		t.Error("Expected ok to be false when extracting tenant ID from a non-TenantContext")
	}

	// Test case: Extract from nil context (should handle gracefully)
	_, ok = GetTenantIDFromContext(nil)

	if ok {
		t.Error("Expected ok to be false when extracting tenant ID from nil context")
	}
}

// TestTenantInterfaces verifies that the interfaces in tenant.go are defined correctly
// This test doesn't actually test functionality but ensures the interfaces have the expected methods
func TestTenantInterfaces(t *testing.T) {
	// Verify TenantService interface methods
	var _ TenantService = &mockTenantService{}

	// Verify TenantAwareModule interface methods
	var _ TenantAwareModule = &mockTenantAwareModule{}
}

// Mock implementations for interface verification

type mockTenantService struct{}

func (m *mockTenantService) GetTenantConfig(tenantID TenantID, section string) (ConfigProvider, error) {
	return nil, nil
}

func (m *mockTenantService) GetTenants() []TenantID {
	return nil
}

func (m *mockTenantService) RegisterTenant(tenantID TenantID, configs map[string]ConfigProvider) error {
	return nil
}

type mockTenantAwareModule struct{}

func (m *mockTenantAwareModule) Name() string {
	return "MockTenantAwareModule"
}

func (m *mockTenantAwareModule) Init(*Application) error {
	return nil
}

func (m *mockTenantAwareModule) Start(ctx context.Context) error {
	return nil
}

func (m *mockTenantAwareModule) Stop(ctx context.Context) error {
	return nil
}

func (m *mockTenantAwareModule) Dependencies() []string {
	return []string{}
}

func (m *mockTenantAwareModule) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{}
}

func (m *mockTenantAwareModule) RequiresServices() []ServiceDependency {
	return []ServiceDependency{}
}

func (m *mockTenantAwareModule) RegisterConfig(app *Application) {
	// Do nothing
}

func (m *mockTenantAwareModule) OnTenantRegistered(tenantID TenantID) {
	// Do nothing
}

func (m *mockTenantAwareModule) OnTenantRemoved(tenantID TenantID) {
	// Do nothing
}
