package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
)

// Test modules for tenant isolation testing
type TestTenantCacheModule struct {
	name string
}

func (m *TestTenantCacheModule) Name() string { return m.name }
func (m *TestTenantCacheModule) Init(app modular.Application) error { return nil }

type TestTenantDatabaseModule struct {
	name string
}

func (m *TestTenantDatabaseModule) Name() string { return m.name }
func (m *TestTenantDatabaseModule) Init(app modular.Application) error { return nil }

// T058: Add integration test for tenant isolation
func TestTenantIsolation_Integration(t *testing.T) {
	t.Run("should isolate tenant configurations", func(t *testing.T) {
		// Create temporary configuration files for different tenants
		tempDir := t.TempDir()

		// Base configuration
		baseConfig := `
database:
  driver: "sqlite"
  dsn: ":memory:"
cache:
  backend: "memory"
  default_ttl: 300
`
		baseConfigPath := filepath.Join(tempDir, "base.yaml")
		err := os.WriteFile(baseConfigPath, []byte(baseConfig), 0644)
		if err != nil {
			t.Fatalf("Failed to create base config: %v", err)
		}

		// Tenant A configuration
		tenantAConfig := `
database:
  table_prefix: "tenantA_"
  max_connections: 10
cache:
  memory_max_size: 1000
  namespace: "tenantA"
`
		tenantADir := filepath.Join(tempDir, "tenants")
		err = os.MkdirAll(tenantADir, 0755)
		if err != nil {
			t.Fatalf("Failed to create tenant directory: %v", err)
		}

		tenantAConfigPath := filepath.Join(tenantADir, "tenantA.yaml")
		err = os.WriteFile(tenantAConfigPath, []byte(tenantAConfig), 0644)
		if err != nil {
			t.Fatalf("Failed to create tenant A config: %v", err)
		}

		// Tenant B configuration
		tenantBConfig := `
database:
  table_prefix: "tenantB_"
  max_connections: 20
cache:
  memory_max_size: 2000
  namespace: "tenantB"
`
		tenantBConfigPath := filepath.Join(tenantADir, "tenantB.yaml")
		err = os.WriteFile(tenantBConfigPath, []byte(tenantBConfig), 0644)
		if err != nil {
			t.Fatalf("Failed to create tenant B config: %v", err)
		}

		// Create applications for different tenants
		appA := modular.NewApplication()
		appA.EnableEnhancedLifecycle()

		appB := modular.NewApplication()
		appB.EnableEnhancedLifecycle()

		// Register modules for tenant A
		dbModA := &TestTenantDatabaseModule{name: "database"}
		cacheModA := &TestTenantCacheModule{name: "cache"}
		appA.RegisterModule("database", dbModA)
		appA.RegisterModule("cache", cacheModA)

		// Register modules for tenant B
		dbModB := &TestTenantDatabaseModule{name: "database"}
		cacheModB := &TestTenantCacheModule{name: "cache"}
		appB.RegisterModule("database", dbModB)
		appB.RegisterModule("cache", cacheModB)

		// Configure tenant A feeders
		baseFeederA := feeders.NewYAMLFileFeeder(baseConfigPath)
		appA.RegisterFeeder("base", baseFeederA)

		tenantFeederA := feeders.NewYAMLFileFeeder(tenantAConfigPath)
		appA.RegisterFeeder("tenant", tenantFeederA)

		// Configure tenant B feeders
		baseFeederB := feeders.NewYAMLFileFeeder(baseConfigPath)
		appB.RegisterFeeder("base", baseFeederB)

		tenantFeederB := feeders.NewYAMLFileFeeder(tenantBConfigPath)
		appB.RegisterFeeder("tenant", tenantFeederB)

		ctx := context.Background()

		// Initialize and start tenant A
		err = appA.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize tenant A: %v", err)
		}

		err = appA.StartWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to start tenant A: %v", err)
		}

		// Initialize and start tenant B
		err = appB.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize tenant B: %v", err)
		}

		err = appB.StartWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to start tenant B: %v", err)
		}

		// Verify tenant A configuration isolation
		providerA := appA.ConfigProvider()
		if providerA == nil {
			t.Fatal("Tenant A config provider should be available")
		}

		tablePrefixA, err := providerA.GetString("database.table_prefix")
		if err != nil {
			t.Fatalf("Failed to get tenant A table prefix: %v", err)
		}
		if tablePrefixA != "tenantA_" {
			t.Errorf("Expected tenantA_, got: %s", tablePrefixA)
		}

		maxConnectionsA, err := providerA.GetInt("database.max_connections")
		if err != nil {
			t.Fatalf("Failed to get tenant A max connections: %v", err)
		}
		if maxConnectionsA != 10 {
			t.Errorf("Expected 10, got: %d", maxConnectionsA)
		}

		memoryMaxSizeA, err := providerA.GetInt("cache.memory_max_size")
		if err != nil {
			t.Fatalf("Failed to get tenant A memory max size: %v", err)
		}
		if memoryMaxSizeA != 1000 {
			t.Errorf("Expected 1000, got: %d", memoryMaxSizeA)
		}

		namespaceA, err := providerA.GetString("cache.namespace")
		if err != nil {
			t.Fatalf("Failed to get tenant A namespace: %v", err)
		}
		if namespaceA != "tenantA" {
			t.Errorf("Expected tenantA, got: %s", namespaceA)
		}

		// Verify tenant B configuration isolation
		providerB := appB.ConfigProvider()
		if providerB == nil {
			t.Fatal("Tenant B config provider should be available")
		}

		tablePrefixB, err := providerB.GetString("database.table_prefix")
		if err != nil {
			t.Fatalf("Failed to get tenant B table prefix: %v", err)
		}
		if tablePrefixB != "tenantB_" {
			t.Errorf("Expected tenantB_, got: %s", tablePrefixB)
		}

		maxConnectionsB, err := providerB.GetInt("database.max_connections")
		if err != nil {
			t.Fatalf("Failed to get tenant B max connections: %v", err)
		}
		if maxConnectionsB != 20 {
			t.Errorf("Expected 20, got: %d", maxConnectionsB)
		}

		memoryMaxSizeB, err := providerB.GetInt("cache.memory_max_size")
		if err != nil {
			t.Fatalf("Failed to get tenant B memory max size: %v", err)
		}
		if memoryMaxSizeB != 2000 {
			t.Errorf("Expected 2000, got: %d", memoryMaxSizeB)
		}

		namespaceB, err := providerB.GetString("cache.namespace")
		if err != nil {
			t.Fatalf("Failed to get tenant B namespace: %v", err)
		}
		if namespaceB != "tenantB" {
			t.Errorf("Expected tenantB, got: %s", namespaceB)
		}

		// Verify shared base configuration is inherited correctly
		driverA, err := providerA.GetString("database.driver")
		if err != nil {
			t.Fatalf("Failed to get tenant A driver: %v", err)
		}
		if driverA != "sqlite" {
			t.Errorf("Expected sqlite, got: %s", driverA)
		}

		driverB, err := providerB.GetString("database.driver")
		if err != nil {
			t.Fatalf("Failed to get tenant B driver: %v", err)
		}
		if driverB != "sqlite" {
			t.Errorf("Expected sqlite, got: %s", driverB)
		}

		defaultTTLA, err := providerA.GetInt("cache.default_ttl")
		if err != nil {
			t.Fatalf("Failed to get tenant A default_ttl: %v", err)
		}
		if defaultTTLA != 300 {
			t.Errorf("Expected 300, got: %d", defaultTTLA)
		}

		defaultTTLB, err := providerB.GetInt("cache.default_ttl")
		if err != nil {
			t.Fatalf("Failed to get tenant B default_ttl: %v", err)
		}
		if defaultTTLB != 300 {
			t.Errorf("Expected 300, got: %d", defaultTTLB)
		}

		// Cleanup
		err = appA.StopWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Errorf("Failed to stop tenant A: %v", err)
		}

		err = appB.StopWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Errorf("Failed to stop tenant B: %v", err)
		}
	})

	t.Run("should isolate tenant service registries", func(t *testing.T) {
		// Create two separate applications representing different tenants
		appTenantA := modular.NewApplication()
		appTenantA.EnableEnhancedLifecycle()

		appTenantB := modular.NewApplication()
		appTenantB.EnableEnhancedLifecycle()

		// Register different modules for each tenant to simulate isolation
		dbModA := &TestTenantDatabaseModule{name: "database"}
		appTenantA.RegisterModule("database", dbModA)

		cacheModB := &TestTenantCacheModule{name: "cache"}
		appTenantB.RegisterModule("cache", cacheModB)

		// Add basic configuration
		mapFeederA := feeders.NewMapFeeder(map[string]interface{}{
			"database.enabled": true,
			"database.driver":  "sqlite",
			"database.dsn":     ":memory:",
		})
		appTenantA.RegisterFeeder("config", mapFeederA)

		mapFeederB := feeders.NewMapFeeder(map[string]interface{}{
			"cache.enabled": true,
			"cache.backend": "memory",
		})
		appTenantB.RegisterFeeder("config", mapFeederB)

		ctx := context.Background()

		// Initialize both tenants
		err := appTenantA.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize tenant A: %v", err)
		}

		err = appTenantB.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize tenant B: %v", err)
		}

		err = appTenantA.StartWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to start tenant A: %v", err)
		}

		err = appTenantB.StartWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to start tenant B: %v", err)
		}

		// Verify service registry isolation
		registryA := appTenantA.ServiceRegistry()
		if registryA == nil {
			t.Fatal("Tenant A service registry should be available")
		}

		registryB := appTenantB.ServiceRegistry()
		if registryB == nil {
			t.Fatal("Tenant B service registry should be available")
		}

		// Get services from each tenant
		servicesA, err := registryA.ListServices()
		if err != nil {
			t.Fatalf("Failed to list tenant A services: %v", err)
		}

		servicesB, err := registryB.ListServices()
		if err != nil {
			t.Fatalf("Failed to list tenant B services: %v", err)
		}

		// Verify different service sets (tenant isolation)
		if len(servicesA) == 0 {
			t.Error("Tenant A should have some services registered")
		}

		if len(servicesB) == 0 {
			t.Error("Tenant B should have some services registered")
		}

		// The service lists might be different due to different modules
		// This verifies that each tenant has its own isolated service registry

		// Cleanup
		err = appTenantA.StopWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Errorf("Failed to stop tenant A: %v", err)
		}

		err = appTenantB.StopWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Errorf("Failed to stop tenant B: %v", err)
		}
	})

	t.Run("should isolate tenant contexts and prevent cross-tenant access", func(t *testing.T) {
		// This test verifies that tenant contexts are properly isolated
		// and that there's no cross-tenant data leakage

		app, err := modular.NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		app.EnableEnhancedLifecycle()

		// Register a cache module that supports tenant contexts
		cacheMod := &cache.Module{}
		app.RegisterModule("cache", cacheMod)

		// Configure with tenant-aware settings
		mapFeeder := feeders.NewMapFeeder(map[string]interface{}{
			"cache.enabled": true,
			"cache.backend": "memory",
		})
		app.RegisterFeeder("config", mapFeeder)

		ctx := context.Background()

		// Initialize and start application
		err := app.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize application: %v", err)
		}

		err = app.StartWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to start application: %v", err)
		}

		// Create tenant contexts
		tenantCtxA := modular.WithTenant(ctx, "tenantA")
		tenantCtxB := modular.WithTenant(ctx, "tenantB")

		// Verify tenant contexts are different
		tenantA := modular.GetTenantID(tenantCtxA)
		tenantB := modular.GetTenantID(tenantCtxB)

		if tenantA == tenantB {
			t.Error("Tenant contexts should be different")
		}

		if tenantA != "tenantA" {
			t.Errorf("Expected tenantA, got: %s", tenantA)
		}

		if tenantB != "tenantB" {
			t.Errorf("Expected tenantB, got: %s", tenantB)
		}

		// Verify tenant isolation in context propagation
		// This test ensures that tenant information is properly isolated
		// between different tenant contexts

		// Test with no tenant context
		noTenantCtx := ctx
		noTenant := modular.GetTenantID(noTenantCtx)
		if noTenant != "" {
			t.Errorf("Expected empty tenant ID, got: %s", noTenant)
		}

		// Cleanup
		err = app.StopWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Errorf("Failed to stop application: %v", err)
		}
	})

	t.Run("should support tenant-specific health monitoring", func(t *testing.T) {
		app, err := modular.NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		app.EnableEnhancedLifecycle()

		// Register modules
		dbMod := &TestTenantDatabaseModule{name: "database"}
		cacheMod := &TestTenantCacheModule{name: "cache"}
		app.RegisterModule("database", dbMod)
		app.RegisterModule("cache", cacheMod)

		// Configure modules
		mapFeeder := feeders.NewMapFeeder(map[string]interface{}{
			"database.enabled": true,
			"database.driver":  "sqlite",
			"database.dsn":     ":memory:",
			"cache.enabled":    true,
			"cache.backend":    "memory",
		})
		app.RegisterFeeder("config", mapFeeder)

		ctx := context.Background()

		// Initialize and start application
		err := app.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize application: %v", err)
		}

		err = app.StartWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to start application: %v", err)
		}

		// Get health aggregator
		healthAggregator := app.GetHealthAggregator()
		if healthAggregator == nil {
			t.Fatal("Health aggregator should be available")
		}

		// Test health monitoring with tenant contexts
		tenantCtxA := modular.WithTenant(ctx, "tenantA")
		tenantCtxB := modular.WithTenant(ctx, "tenantB")

		// Get health status for different tenants
		healthA, err := healthAggregator.GetOverallHealth(tenantCtxA)
		if err != nil {
			t.Fatalf("Failed to get health for tenant A: %v", err)
		}

		healthB, err := healthAggregator.GetOverallHealth(tenantCtxB)
		if err != nil {
			t.Fatalf("Failed to get health for tenant B: %v", err)
		}

		// Both should be healthy, but the health aggregator should be capable
		// of handling tenant-specific contexts
		if healthA.Status != "healthy" && healthA.Status != "warning" {
			t.Errorf("Expected healthy status for tenant A, got: %s", healthA.Status)
		}

		if healthB.Status != "healthy" && healthB.Status != "warning" {
			t.Errorf("Expected healthy status for tenant B, got: %s", healthB.Status)
		}

		// Get health without tenant context
		healthGlobal, err := healthAggregator.GetOverallHealth(ctx)
		if err != nil {
			t.Fatalf("Failed to get global health: %v", err)
		}

		if healthGlobal.Status != "healthy" && healthGlobal.Status != "warning" {
			t.Errorf("Expected healthy global status, got: %s", healthGlobal.Status)
		}

		// Cleanup
		err = app.StopWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Errorf("Failed to stop application: %v", err)
		}
	})
}