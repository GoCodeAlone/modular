//go:build failing_test

package modular

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithServiceScopeOption(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_define_with_service_scope_option_function",
			testFunc: func(t *testing.T) {
				// Test that WithServiceScope function exists
				option := WithServiceScope("test-service", ServiceScopeSingleton)
				assert.NotNil(t, option, "WithServiceScope should return a service registry option")
			},
		},
		{
			name: "should_accept_service_scope_configuration",
			testFunc: func(t *testing.T) {
				// Test that WithServiceScope accepts different scope configurations
				config := ServiceScopeConfig{
					Scope:           ServiceScopeScoped,
					ScopeKey:        "tenant_id",
					MaxInstances:    100,
					InstanceTimeout: "5m",
				}

				option := WithServiceScopeConfig("database", config)
				assert.NotNil(t, option, "WithServiceScopeConfig should accept detailed configuration")
			},
		},
		{
			name: "should_apply_option_to_service_registry",
			testFunc: func(t *testing.T) {
				// Test that WithServiceScope option can be applied to service registry
				registry := NewServiceRegistry()
				option := WithServiceScope("cache", ServiceScopeTransient)

				err := registry.ApplyOption(option)
				assert.NoError(t, err, "Should apply WithServiceScope option to registry")
			},
		},
		{
			name: "should_configure_service_scoping_behavior",
			testFunc: func(t *testing.T) {
				// Test that service registry respects scope configuration
				registry := NewServiceRegistry()

				err := registry.ApplyOption(WithServiceScope("singleton-service", ServiceScopeSingleton))
				require.NoError(t, err, "Should apply singleton scope")

				err = registry.ApplyOption(WithServiceScope("transient-service", ServiceScopeTransient))
				require.NoError(t, err, "Should apply transient scope")

				// Check that scopes are configured correctly
				singletonScope := registry.GetServiceScope("singleton-service")
				assert.Equal(t, ServiceScopeSingleton, singletonScope, "Singleton service should have singleton scope")

				transientScope := registry.GetServiceScope("transient-service")
				assert.Equal(t, ServiceScopeTransient, transientScope, "Transient service should have transient scope")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestServiceScopeOptionBehavior(t *testing.T) {
	tests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T)
	}{
		{
			name:        "should_enforce_singleton_behavior",
			description: "Services configured with singleton scope should return the same instance",
			testFunc: func(t *testing.T) {
				registry := NewServiceRegistry()
				registry.ApplyOption(WithServiceScope("singleton-service", ServiceScopeSingleton))

				// Register a service factory
				registry.Register("singleton-service", func() interface{} {
					return &testService{ID: time.Now().UnixNano()}
				})

				// Get service instances
				instance1, err := registry.Get("singleton-service")
				require.NoError(t, err, "Should get service instance")

				instance2, err := registry.Get("singleton-service")
				require.NoError(t, err, "Should get service instance")

				// Should be the same instance
				service1 := instance1.(*testService)
				service2 := instance2.(*testService)
				assert.Equal(t, service1.ID, service2.ID, "Singleton services should return the same instance")
			},
		},
		{
			name:        "should_enforce_transient_behavior",
			description: "Services configured with transient scope should return new instances",
			testFunc: func(t *testing.T) {
				registry := NewServiceRegistry()
				registry.ApplyOption(WithServiceScope("transient-service", ServiceScopeTransient))

				// Register a service factory
				registry.Register("transient-service", func() interface{} {
					return &testService{ID: time.Now().UnixNano()}
				})

				// Get service instances with small delay to ensure different timestamps
				instance1, err := registry.Get("transient-service")
				require.NoError(t, err, "Should get service instance")

				time.Sleep(1 * time.Millisecond)
				instance2, err := registry.Get("transient-service")
				require.NoError(t, err, "Should get service instance")

				// Should be different instances
				service1 := instance1.(*testService)
				service2 := instance2.(*testService)
				assert.NotEqual(t, service1.ID, service2.ID, "Transient services should return different instances")
			},
		},
		{
			name:        "should_enforce_scoped_behavior",
			description: "Services configured with scoped scope should return same instance within scope",
			testFunc: func(t *testing.T) {
				registry := NewServiceRegistry()
				config := ServiceScopeConfig{
					Scope:    ServiceScopeScoped,
					ScopeKey: "tenant_id",
				}
				registry.ApplyOption(WithServiceScopeConfig("scoped-service", config))

				// Register a service factory
				registry.Register("scoped-service", func() interface{} {
					return &testService{ID: time.Now().UnixNano()}
				})

				// Get service instances within same scope
				ctx1 := WithScopeContext(context.Background(), "tenant_id", "tenant-a")
				instance1, err := registry.GetWithContext(ctx1, "scoped-service")
				require.NoError(t, err, "Should get scoped service instance")

				instance2, err := registry.GetWithContext(ctx1, "scoped-service")
				require.NoError(t, err, "Should get scoped service instance")

				// Should be the same instance within scope
				service1 := instance1.(*testService)
				service2 := instance2.(*testService)
				assert.Equal(t, service1.ID, service2.ID, "Scoped services should return same instance within scope")

				// Get service instance from different scope
				ctx2 := WithScopeContext(context.Background(), "tenant_id", "tenant-b")
				instance3, err := registry.GetWithContext(ctx2, "scoped-service")
				require.NoError(t, err, "Should get scoped service instance")

				// Should be different instance in different scope
				service3 := instance3.(*testService)
				assert.NotEqual(t, service1.ID, service3.ID, "Scoped services should return different instances across scopes")
			},
		},
		{
			name:        "should_respect_max_instances_limit",
			description: "Service scope configuration should respect max instances limit",
			testFunc: func(t *testing.T) {
				registry := NewServiceRegistry()
				config := ServiceScopeConfig{
					Scope:        ServiceScopeTransient,
					MaxInstances: 2, // Limit to 2 instances
				}
				registry.ApplyOption(WithServiceScopeConfig("limited-service", config))

				// Register a service factory
				registry.Register("limited-service", func() interface{} {
					return &testService{ID: time.Now().UnixNano()}
				})

				// Get instances up to the limit
				instance1, err := registry.Get("limited-service")
				assert.NoError(t, err, "Should get first instance")
				assert.NotNil(t, instance1, "First instance should not be nil")

				instance2, err := registry.Get("limited-service")
				assert.NoError(t, err, "Should get second instance")
				assert.NotNil(t, instance2, "Second instance should not be nil")

				// Attempt to get third instance should fail or return existing
				instance3, err := registry.Get("limited-service")
				if err != nil {
					assert.Contains(t, err.Error(), "max instances", "Error should mention max instances limit")
				} else {
					// If no error, should return one of the existing instances
					service3 := instance3.(*testService)
					service1ID := instance1.(*testService).ID
					service2ID := instance2.(*testService).ID
					assert.True(t, service3.ID == service1ID || service3.ID == service2ID,
						"Third instance should reuse existing instance when limit reached")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

// Helper types for testing
type testService struct {
	ID int64
}
