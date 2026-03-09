package reverseproxy

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
)

func TestDebugServiceInit(t *testing.T) {
	ctx := &ReverseProxyBDDTestContext{}

	// Test basic setup
	err := ctx.iHaveAModularApplicationWithReverseProxyModuleConfigured()
	if err != nil {
		t.Fatalf("Basic setup failed: %v", err)
	}

	t.Logf("Basic setup complete")
	t.Logf("ctx.app != nil: %v", ctx.app != nil)
	t.Logf("ctx.config != nil: %v", ctx.config != nil)
	t.Logf("ctx.module != nil: %v", ctx.module != nil)

	// Try DNS setup
	err = ctx.iHaveAReverseProxyWithHealthChecksConfiguredForDNSResolution()
	if err != nil {
		t.Fatalf("DNS setup failed: %v", err)
	}

	t.Logf("DNS setup complete")
	t.Logf("ctx.service != nil: %v", ctx.service != nil)
	if ctx.service != nil {
		t.Logf("ctx.service.healthChecker != nil: %v", ctx.service.healthChecker != nil)
		t.Logf("ctx.service.config != nil: %v", ctx.service.config != nil)
		if ctx.service.config != nil {
			t.Logf("Health check enabled: %v", ctx.service.config.HealthCheck.Enabled)
		}
	}

	// Wait a bit for initialization
	time.Sleep(200 * time.Millisecond)

	// Check again
	if ctx.service != nil {
		t.Logf("After wait - ctx.service.healthChecker != nil: %v", ctx.service.healthChecker != nil)
		if ctx.service.healthChecker != nil {
			status := ctx.service.healthChecker.GetHealthStatus()
			t.Logf("Health status: %v", status != nil)
			if status != nil {
				t.Logf("Number of backends in status: %d", len(status))
				for id, s := range status {
					t.Logf("Backend %s: healthy=%v", id, s.Healthy)
				}
			}
		}
	}
}

// TestRouterServiceInterfaceCompatibility verifies that testRouter implements routerService
func TestRouterServiceInterfaceCompatibility(t *testing.T) {
	// Create a test router instance
	router := &testRouter{
		routes: make(map[string]http.HandlerFunc),
	}

	// Test interface assertion
	var routerSvc routerService = router
	assert.NotNil(t, routerSvc, "testRouter should implement routerService interface")

	// Test the type assertion that's failing in the constructor
	services := map[string]any{
		"router": router,
	}

	handleFuncSvc, ok := services["router"].(routerService)
	if !ok {
		t.Errorf("Failed to cast router service to routerService interface")
		t.Errorf("Router type: %T", services["router"])
		return
	}

	assert.NotNil(t, handleFuncSvc, "Router service should not be nil after casting")
	assert.True(t, ok, "Router service should cast successfully to routerService")

	fmt.Printf("Router service type: %T\n", handleFuncSvc)
	fmt.Printf("Router service methods available: Handle, HandleFunc, Mount, Use, ServeHTTP\n")
}

// TestBDDSetupDebug mimics the BDD setup to find the router interface issue
func TestBDDSetupDebug(t *testing.T) {
	// Create application
	app, err := modular.NewApplication(modular.WithLogger(&testLogger{}))
	assert.NoError(t, err)

	// Register router service
	router := &testRouter{
		routes: make(map[string]http.HandlerFunc),
	}
	err = app.RegisterService("router", router)
	assert.NoError(t, err)

	// Get router service back
	var retrievedRouter *testRouter
	err = app.GetService("router", &retrievedRouter)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedRouter)

	// Test interface assertion on retrieved router
	var routerSvc routerService = retrievedRouter
	assert.NotNil(t, routerSvc)

	// Test the exact pattern used in BDD constructor
	services := map[string]any{
		"router": retrievedRouter,
	}

	handleFuncSvc, ok := services["router"].(routerService)
	if !ok {
		t.Errorf("Failed to cast retrieved router service to routerService interface")
		t.Errorf("Retrieved router type: %T", services["router"])
		t.Errorf("Router is nil: %v", retrievedRouter == nil)
		if retrievedRouter != nil {
			t.Errorf("Router routes is nil: %v", retrievedRouter.routes == nil)
		}
		return
	}

	assert.NotNil(t, handleFuncSvc, "Retrieved router service should not be nil after casting")
	assert.True(t, ok, "Retrieved router service should cast successfully to routerService")

	fmt.Printf("Retrieved router service type: %T\n", handleFuncSvc)

	// Now test the actual module constructor
	module := NewModule()
	constructor := module.Constructor()

	constructedModule, err := constructor(app, services)
	if err != nil {
		t.Errorf("Constructor failed: %v", err)
		return
	}

	assert.NotNil(t, constructedModule, "Constructed module should not be nil")
	fmt.Printf("Constructor succeeded with router type: %T\n", retrievedRouter)
}
