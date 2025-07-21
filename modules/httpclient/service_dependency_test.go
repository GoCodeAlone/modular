package httpclient

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/CrisisTextLine/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Use the HTTPDoer interface from the httpclient service package
// This avoids duplication and uses the same interface the module provides

// TestHTTPClientInterface tests that http.Client implements the HTTPDoer interface
func TestHTTPClientInterface(t *testing.T) {
	client := &http.Client{}

	// Test that http.Client implements HTTPDoer interface
	var doer HTTPDoer = client
	assert.NotNil(t, doer, "http.Client should implement HTTPDoer interface")

	// Test reflection-based interface checking (this is what the framework uses)
	clientType := reflect.TypeOf(client)
	doerInterface := reflect.TypeOf((*HTTPDoer)(nil)).Elem()

	assert.True(t, clientType.Implements(doerInterface),
		"http.Client should implement HTTPDoer interface via reflection")
}

// TestServiceDependencyResolution tests interface-based service resolution
func TestServiceDependencyResolution(t *testing.T) {
	// Create test application with proper config provider and logger
	app := modular.NewStdApplication(modular.NewStdConfigProvider(nil), &testLogger{t: t})

	// Register httpclient module
	httpClientModule := NewHTTPClientModule()
	app.RegisterModule(httpClientModule)

	// Register consumer module that depends on httpclient
	var consumerModule modular.Module = NewTestConsumerModule()
	app.RegisterModule(consumerModule)

	// Initialize the application
	err := app.Init()
	require.NoError(t, err)

	// Test that httpclient module provides the expected services
	serviceAware, ok := httpClientModule.(modular.ServiceAware)
	require.True(t, ok, "httpclient should be ServiceAware")

	providedServices := serviceAware.ProvidesServices()
	require.Len(t, providedServices, 2, "httpclient should provide 2 services")

	// Verify service names and that the http.Client implements HTTPDoer
	serviceNames := make(map[string]bool)
	var httpClient *http.Client
	for _, svc := range providedServices {
		serviceNames[svc.Name] = true
		if svc.Name == "httpclient" {
			httpClient = svc.Instance.(*http.Client)
		}
	}
	assert.True(t, serviceNames["httpclient"], "should provide 'httpclient' service")
	assert.True(t, serviceNames["httpclient-service"], "should provide 'httpclient-service' service")

	// Test that the HTTP client implements the HTTPDoer interface
	require.NotNil(t, httpClient)
	var httpDoer HTTPDoer = httpClient
	assert.NotNil(t, httpDoer, "http.Client should implement HTTPDoer interface")

	// Test that the consumer module can be created and has the correct dependency structure
	consumerServiceAware, ok := consumerModule.(modular.ServiceAware)
	require.True(t, ok, "consumer should be ServiceAware")

	consumerDependencies := consumerServiceAware.RequiresServices()
	require.Len(t, consumerDependencies, 1, "consumer should require 1 service")

	// Check that the dependencies are correctly configured
	depMap := make(map[string]modular.ServiceDependency)
	for _, dep := range consumerDependencies {
		depMap[dep.Name] = dep
	}

	// Verify httpclient dependency (interface-based)
	httpclientDep, exists := depMap["httpclient"]
	assert.True(t, exists, "httpclient dependency should exist")
	assert.True(t, httpclientDep.MatchByInterface, "httpclient should use interface-based matching")
}

// TestConsumerModule simulates a module that depends on httpclient service via interface
type TestConsumerModule struct {
	httpClient HTTPDoer
}

func NewTestConsumerModule() *TestConsumerModule {
	return &TestConsumerModule{}
}

func (m *TestConsumerModule) Name() string {
	return "consumer"
}

func (m *TestConsumerModule) Init(app modular.Application) error {
	return nil
}

func (m *TestConsumerModule) ProvidesServices() []modular.ServiceProvider {
	return nil
}

func (m *TestConsumerModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:               "httpclient",
			Required:           false,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*HTTPDoer)(nil)).Elem(),
		},
	}
}

func (m *TestConsumerModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		// Get interface-based service
		if httpClient, ok := services["httpclient"]; ok {
			if doer, ok := httpClient.(HTTPDoer); ok {
				m.httpClient = doer
			}
		}

		return m, nil
	}
}

// testLogger is a simple test logger implementation
type testLogger struct {
	t *testing.T
}

func (l *testLogger) Debug(msg string, keyvals ...interface{}) {
	l.t.Logf("DEBUG: %s %v", msg, keyvals)
}

func (l *testLogger) Info(msg string, keyvals ...interface{}) {
	l.t.Logf("INFO: %s %v", msg, keyvals)
}

func (l *testLogger) Warn(msg string, keyvals ...interface{}) {
	l.t.Logf("WARN: %s %v", msg, keyvals)
}

func (l *testLogger) Error(msg string, keyvals ...interface{}) {
	l.t.Logf("ERROR: %s %v", msg, keyvals)
}
