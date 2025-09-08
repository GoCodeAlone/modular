package integration

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	modular "github.com/GoCodeAlone/modular"
)

// TestMultiTenancyIsolationUnderLoad tests T025: Integration multi-tenancy isolation under load
// This test verifies that tenant data and operations remain isolated even under concurrent load.
func TestMultiTenancyIsolationUnderLoad(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	// Create application with tenant service
	app := modular.NewStdApplication(modular.NewStdConfigProvider(&struct{}{}), logger)
	
	// Register tenant service
	tenantService := modular.NewStandardTenantService(logger)
	if err := app.RegisterService("tenantService", tenantService); err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}
	
	// Register a simple tenant config loader
	configLoader := &testTenantConfigLoader{}
	if err := app.RegisterService("tenantConfigLoader", configLoader); err != nil {
		t.Fatalf("Failed to register tenant config loader: %v", err)
	}
	
	// Register tenant-aware module
	tenantModule := &testTenantAwareModule{}
	app.RegisterModule(tenantModule)
	
	// Initialize application
	err := app.Init()
	if err != nil {
		t.Fatalf("Application initialization failed: %v", err)
	}
	
	// Register multiple tenants
	tenantIDs := []modular.TenantID{"tenant1", "tenant2", "tenant3", "tenant4"}
	for _, tenantID := range tenantIDs {
		err = tenantService.RegisterTenant(tenantID, map[string]modular.ConfigProvider{
			"test": modular.NewStdConfigProvider(map[string]interface{}{
				"name": fmt.Sprintf("Tenant %s", tenantID),
			}),
		})
		if err != nil {
			t.Fatalf("Failed to register tenant %s: %v", tenantID, err)
		}
	}
	
	// Test concurrent operations to verify isolation
	const numOperationsPerTenant = 100
	const numWorkers = 10
	
	var wg sync.WaitGroup
	results := make(map[string][]string)
	resultsMutex := sync.Mutex{}
	
	// Start concurrent workers for each tenant
	for _, tenantID := range tenantIDs {
		for worker := 0; worker < numWorkers; worker++ {
			wg.Add(1)
			go func(tid modular.TenantID, workerID int) {
				defer wg.Done()
				
				for op := 0; op < numOperationsPerTenant; op++ {
					// Simulate tenant-specific operations
					ctx := modular.NewTenantContext(context.Background(), tid)
					
					// Use tenant module with specific context
					result := tenantModule.ProcessTenantData(ctx, fmt.Sprintf("worker%d_op%d", workerID, op))
					
					// Store results per tenant
					resultsMutex.Lock()
					tenantKey := string(tid)
					if results[tenantKey] == nil {
						results[tenantKey] = make([]string, 0)
					}
					results[tenantKey] = append(results[tenantKey], result)
					resultsMutex.Unlock()
				}
			}(tenantID, worker)
		}
	}
	
	// Wait for all operations to complete
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()
	
	select {
	case <-done:
		t.Log("✅ All concurrent operations completed")
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out waiting for concurrent operations")
	}
	
	// Verify isolation: each tenant should have exactly the expected number of results
	expectedResultsPerTenant := numWorkers * numOperationsPerTenant
	
	for _, tenantID := range tenantIDs {
		tenantKey := string(tenantID)
		tenantResults, exists := results[tenantKey]
		if !exists {
			t.Errorf("No results found for tenant %s", tenantID)
			continue
		}
		
		if len(tenantResults) != expectedResultsPerTenant {
			t.Errorf("Tenant %s: expected %d results, got %d", tenantID, expectedResultsPerTenant, len(tenantResults))
		}
		
		// Verify all results are properly prefixed with tenant ID (indicating isolation)
		for _, result := range tenantResults {
			expectedPrefix := fmt.Sprintf("[%s]", tenantID)
			if len(result) < len(expectedPrefix) || result[:len(expectedPrefix)] != expectedPrefix {
				t.Errorf("Tenant %s: result not properly isolated: %s", tenantID, result)
				break
			}
		}
	}
	
	// Verify no cross-tenant contamination
	for _, tenantID := range tenantIDs {
		tenantKey := string(tenantID)
		tenantResults := results[tenantKey]
		for _, result := range tenantResults {
			for _, otherTenantID := range tenantIDs {
				if otherTenantID != tenantID {
					contaminationPrefix := fmt.Sprintf("[%s]", otherTenantID)
					if len(result) >= len(contaminationPrefix) && result[:len(contaminationPrefix)] == contaminationPrefix {
						t.Errorf("Cross-tenant contamination detected: result %s in tenant %s contains data from tenant %s", result, tenantID, otherTenantID)
					}
				}
			}
		}
	}
	
	t.Logf("✅ Multi-tenancy isolation verified under load")
	t.Logf("   - %d tenants", len(tenantIDs))
	t.Logf("   - %d workers per tenant", numWorkers)
	t.Logf("   - %d operations per worker", numOperationsPerTenant)
	t.Logf("   - Total operations: %d", len(tenantIDs)*numWorkers*numOperationsPerTenant)
}

// testTenantConfigLoader provides a simple tenant config loader for testing
type testTenantConfigLoader struct{}

func (l *testTenantConfigLoader) LoadTenantConfig(tenantID string, configSections map[string]interface{}) error {
	// Simple config loader that doesn't actually load anything
	return nil
}

// testTenantAwareModule is a module that processes tenant-specific data
type testTenantAwareModule struct {
	name string
}

func (m *testTenantAwareModule) Name() string {
	if m.name == "" {
		return "testTenantModule"
	}
	return m.name
}

func (m *testTenantAwareModule) Init(app modular.Application) error {
	return nil
}

// ProcessTenantData simulates tenant-aware data processing
func (m *testTenantAwareModule) ProcessTenantData(ctx context.Context, data string) string {
	// Extract tenant ID from context
	tenantID, ok := modular.GetTenantIDFromContext(ctx)
	if !ok {
		tenantID = "unknown"
	}
	
	// Return tenant-prefixed result to verify isolation
	return fmt.Sprintf("[%s] processed: %s", tenantID, data)
}

// Implement TenantAwareModule interface if it exists in the framework
func (m *testTenantAwareModule) OnTenantRegistered(tenantID modular.TenantID) {
	// Handle tenant registration
}