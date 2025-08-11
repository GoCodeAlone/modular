package reverseproxy

import (
	"net/http"
	"testing"

	"github.com/CrisisTextLine/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReverseProxyServiceDependencyResolution tests that the reverseproxy module
// can receive HTTP client services via interface-based matching
func TestReverseProxyServiceDependencyResolution(t *testing.T) {
	// Use t.Setenv to isolate environment variables in tests
	t.Setenv("REQUEST_TIMEOUT", "10s")

	// Test 1: Interface-based service resolution
	t.Run("InterfaceBasedServiceResolution", func(t *testing.T) {
		app := modular.NewStdApplication(modular.NewStdConfigProvider(nil), &testLoggerDep{t: t})

		// Create mock HTTP client
		mockClient := &http.Client{}

		// Create a mock router service that satisfies the routerService interface
		mockRouter := &testRouter{
			routes: make(map[string]http.HandlerFunc),
		}

		// Register services manually for testing
		err := app.RegisterService("router", mockRouter)
		require.NoError(t, err)

		err = app.RegisterService("httpclient", mockClient)
		require.NoError(t, err)

		// Create reverseproxy module
		reverseProxyModule := NewModule()
		app.RegisterModule(reverseProxyModule)

		// Initialize application
		err = app.Init()
		require.NoError(t, err)

		// Verify the module received the httpclient service
		assert.NotNil(t, reverseProxyModule.httpClient, "HTTP client should be set")
		assert.Same(t, mockClient, reverseProxyModule.httpClient, "Should use the provided HTTP client")
	})

	// Test 2: No HTTP client service (default client creation)
	t.Run("DefaultClientCreation", func(t *testing.T) {
		app := modular.NewStdApplication(modular.NewStdConfigProvider(nil), &testLoggerDep{t: t})

		// Create a mock router service that satisfies the routerService interface
		mockRouter := &testRouter{
			routes: make(map[string]http.HandlerFunc),
		}

		// Register only router service, no HTTP client services
		err := app.RegisterService("router", mockRouter)
		require.NoError(t, err)

		// Create reverseproxy module
		reverseProxyModule := NewModule()
		app.RegisterModule(reverseProxyModule)

		// Initialize application
		err = app.Init()
		require.NoError(t, err)

		// Verify the module created a default HTTP client
		assert.NotNil(t, reverseProxyModule.httpClient, "HTTP client should be created as default")
	})
}

// TestServiceDependencyConfiguration tests that the reverseproxy module declares the correct dependencies
func TestServiceDependencyConfiguration(t *testing.T) {
	module := NewModule()

	// Check that module implements ServiceAware
	var serviceAware modular.ServiceAware = module
	require.NotNil(t, serviceAware, "reverseproxy module should implement ServiceAware")

	// Get service dependencies
	dependencies := serviceAware.RequiresServices()
	require.Len(t, dependencies, 3, "reverseproxy should declare 3 service dependencies")

	// Map dependencies by name for easy checking
	depMap := make(map[string]modular.ServiceDependency)
	for _, dep := range dependencies {
		depMap[dep.Name] = dep
	}

	// Check router dependency (required, interface-based)
	routerDep, exists := depMap["router"]
	assert.True(t, exists, "router dependency should exist")
	assert.True(t, routerDep.Required, "router dependency should be required")
	assert.True(t, routerDep.MatchByInterface, "router dependency should use interface matching")

	// Check httpclient dependency (optional, name-based)
	httpclientDep, exists := depMap["httpclient"]
	assert.True(t, exists, "httpclient dependency should exist")
	assert.False(t, httpclientDep.Required, "httpclient dependency should be optional")
	assert.False(t, httpclientDep.MatchByInterface, "httpclient dependency should use name-based matching")
	assert.Nil(t, httpclientDep.SatisfiesInterface, "httpclient dependency should not specify interface for name-based matching")

	// Check featureFlagEvaluator dependency (optional, interface-based)
	featureFlagDep, exists := depMap["featureFlagEvaluator"]
	assert.True(t, exists, "featureFlagEvaluator dependency should exist")
	assert.False(t, featureFlagDep.Required, "featureFlagEvaluator dependency should be optional")
	assert.True(t, featureFlagDep.MatchByInterface, "featureFlagEvaluator dependency should use interface matching")
	assert.NotNil(t, featureFlagDep.SatisfiesInterface, "featureFlagEvaluator dependency should specify interface")
}

// testLoggerDep is a simple test logger implementation
type testLoggerDep struct {
	t *testing.T
}

func (l *testLoggerDep) Debug(msg string, keyvals ...interface{}) {
	l.t.Logf("DEBUG: %s %v", msg, keyvals)
}

func (l *testLoggerDep) Info(msg string, keyvals ...interface{}) {
	l.t.Logf("INFO: %s %v", msg, keyvals)
}

func (l *testLoggerDep) Warn(msg string, keyvals ...interface{}) {
	l.t.Logf("WARN: %s %v", msg, keyvals)
}

func (l *testLoggerDep) Error(msg string, keyvals ...interface{}) {
	l.t.Logf("ERROR: %s %v", msg, keyvals)
}
