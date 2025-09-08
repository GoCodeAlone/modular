
package modular

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceScope(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_define_service_scope_constants",
			testFunc: func(t *testing.T) {
				// Test that ServiceScope constants are defined
				assert.Equal(t, "singleton", string(ServiceScopeSingleton), "ServiceScopeSingleton should be 'singleton'")
				assert.Equal(t, "transient", string(ServiceScopeTransient), "ServiceScopeTransient should be 'transient'")
				assert.Equal(t, "scoped", string(ServiceScopeScoped), "ServiceScopeScoped should be 'scoped'")
				assert.Equal(t, "factory", string(ServiceScopeFactory), "ServiceScopeFactory should be 'factory'")
			},
		},
		{
			name: "should_support_string_conversion",
			testFunc: func(t *testing.T) {
				// Test that ServiceScope can be converted to string
				scope := ServiceScopeSingleton
				str := scope.String()
				assert.Equal(t, "singleton", str, "ServiceScope should convert to string")
			},
		},
		{
			name: "should_parse_from_string",
			testFunc: func(t *testing.T) {
				// Test that ServiceScope can be parsed from string
				scope, err := ParseServiceScope("singleton")
				assert.NoError(t, err, "Should parse valid service scope")
				assert.Equal(t, ServiceScopeSingleton, scope, "Should parse singleton correctly")

				scope, err = ParseServiceScope("transient")
				assert.NoError(t, err, "Should parse valid service scope")
				assert.Equal(t, ServiceScopeTransient, scope, "Should parse transient correctly")

				scope, err = ParseServiceScope("scoped")
				assert.NoError(t, err, "Should parse valid service scope")
				assert.Equal(t, ServiceScopeScoped, scope, "Should parse scoped correctly")

				scope, err = ParseServiceScope("factory")
				assert.NoError(t, err, "Should parse valid service scope")
				assert.Equal(t, ServiceScopeFactory, scope, "Should parse factory correctly")
			},
		},
		{
			name: "should_handle_invalid_scope_strings",
			testFunc: func(t *testing.T) {
				// Test that invalid scope strings return error
				_, err := ParseServiceScope("invalid")
				assert.Error(t, err, "Should return error for invalid scope")
				assert.Contains(t, err.Error(), "invalid service scope", "Error should mention invalid scope")

				_, err = ParseServiceScope("")
				assert.Error(t, err, "Should return error for empty scope")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestServiceScopeValidation(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_validate_service_scope",
			testFunc: func(t *testing.T) {
				// Test that valid scopes pass validation
				assert.True(t, ServiceScopeSingleton.IsValid(), "Singleton should be valid")
				assert.True(t, ServiceScopeTransient.IsValid(), "Transient should be valid")
				assert.True(t, ServiceScopeScoped.IsValid(), "Scoped should be valid")
				assert.True(t, ServiceScopeFactory.IsValid(), "Factory should be valid")
			},
		},
		{
			name: "should_identify_default_scope",
			testFunc: func(t *testing.T) {
				// Test that we can identify the default scope
				defaultScope := GetDefaultServiceScope()
				assert.Equal(t, ServiceScopeSingleton, defaultScope, "Default scope should be singleton")
			},
		},
		{
			name: "should_check_if_scope_allows_multiple_instances",
			testFunc: func(t *testing.T) {
				// Test scope behavior properties
				assert.False(t, ServiceScopeSingleton.AllowsMultipleInstances(), "Singleton should not allow multiple instances")
				assert.True(t, ServiceScopeTransient.AllowsMultipleInstances(), "Transient should allow multiple instances")
				assert.True(t, ServiceScopeScoped.AllowsMultipleInstances(), "Scoped should allow multiple instances")
				assert.True(t, ServiceScopeFactory.AllowsMultipleInstances(), "Factory should allow multiple instances")
			},
		},
		{
			name: "should_check_if_scope_is_cacheable",
			testFunc: func(t *testing.T) {
				// Test if instances should be cached
				assert.True(t, ServiceScopeSingleton.IsCacheable(), "Singleton should be cacheable")
				assert.False(t, ServiceScopeTransient.IsCacheable(), "Transient should not be cacheable")
				assert.True(t, ServiceScopeScoped.IsCacheable(), "Scoped should be cacheable")
				assert.False(t, ServiceScopeFactory.IsCacheable(), "Factory should not be cacheable")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestServiceScopeDescription(t *testing.T) {
	tests := []struct {
		scope          ServiceScope
		expectedDesc   string
		expectedDetail string
	}{
		{
			scope:        ServiceScopeSingleton,
			expectedDesc: "Single instance shared across the application",
			expectedDetail: "One instance is created and reused for all requests",
		},
		{
			scope:        ServiceScopeTransient,
			expectedDesc: "New instance created for each request",
			expectedDetail: "A new instance is created every time the service is requested",
		},
		{
			scope:        ServiceScopeScoped,
			expectedDesc: "Single instance per scope (e.g., request, session)",
			expectedDetail: "One instance per defined scope boundary",
		},
		{
			scope:        ServiceScopeFactory,
			expectedDesc: "Factory method called for each request",
			expectedDetail: "A factory function is invoked to create instances",
		},
	}

	for _, tt := range tests {
		t.Run("should_provide_description_for_"+tt.scope.String(), func(t *testing.T) {
			desc := tt.scope.Description()
			assert.Equal(t, tt.expectedDesc, desc, "Should provide correct description")

			detail := tt.scope.DetailedDescription()
			assert.Equal(t, tt.expectedDetail, detail, "Should provide correct detailed description")
		})
	}
}

func TestServiceScopeComparison(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_compare_service_scopes",
			testFunc: func(t *testing.T) {
				// Test scope equality
				assert.True(t, ServiceScopeSingleton.Equals(ServiceScopeSingleton), "Same scopes should be equal")
				assert.False(t, ServiceScopeSingleton.Equals(ServiceScopeTransient), "Different scopes should not be equal")
			},
		},
		{
			name: "should_determine_scope_compatibility",
			testFunc: func(t *testing.T) {
				// Test if scopes are compatible for dependency injection
				assert.True(t, ServiceScopeSingleton.IsCompatibleWith(ServiceScopeTransient), "Singleton can depend on transient")
				assert.True(t, ServiceScopeScoped.IsCompatibleWith(ServiceScopeTransient), "Scoped can depend on transient")
				assert.False(t, ServiceScopeTransient.IsCompatibleWith(ServiceScopeSingleton), "Transient should not depend on singleton directly")
			},
		},
		{
			name: "should_order_scopes_by_lifetime",
			testFunc: func(t *testing.T) {
				// Test scope ordering by lifetime (longest to shortest)
				scopes := []ServiceScope{ServiceScopeTransient, ServiceScopeSingleton, ServiceScopeScoped, ServiceScopeFactory}
				ordered := OrderScopesByLifetime(scopes)
				
				assert.Equal(t, ServiceScopeSingleton, ordered[0], "Singleton should have longest lifetime")
				assert.Equal(t, ServiceScopeScoped, ordered[1], "Scoped should be second longest")
				// Transient and Factory should be shorter-lived
				assert.Contains(t, []ServiceScope{ServiceScopeTransient, ServiceScopeFactory}, ordered[2])
				assert.Contains(t, []ServiceScope{ServiceScopeTransient, ServiceScopeFactory}, ordered[3])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestServiceScopeConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_create_scope_configuration",
			testFunc: func(t *testing.T) {
				// Test creating scope configuration
				config := ServiceScopeConfig{
					Scope:           ServiceScopeScoped,
					ScopeKey:        "request_id",
					MaxInstances:    100,
					InstanceTimeout: "5m",
				}
				assert.Equal(t, ServiceScopeScoped, config.Scope, "ScopeConfig should store scope")
				assert.Equal(t, "request_id", config.ScopeKey, "ScopeConfig should store scope key")
			},
		},
		{
			name: "should_validate_scope_configuration",
			testFunc: func(t *testing.T) {
				// Test scope configuration validation
				validConfig := ServiceScopeConfig{
					Scope:           ServiceScopeScoped,
					ScopeKey:        "tenant_id",
					MaxInstances:    50,
					InstanceTimeout: "10m",
				}
				assert.True(t, validConfig.IsValid(), "Valid config should pass validation")

				invalidConfig := ServiceScopeConfig{
					Scope:        ServiceScopeScoped,
					ScopeKey:     "", // Empty scope key for scoped service
					MaxInstances: -1, // Negative max instances
				}
				assert.False(t, invalidConfig.IsValid(), "Invalid config should fail validation")
			},
		},
		{
			name: "should_provide_scope_defaults",
			testFunc: func(t *testing.T) {
				// Test default configurations for different scopes
				singletonDefaults := GetDefaultScopeConfig(ServiceScopeSingleton)
				assert.Equal(t, ServiceScopeSingleton, singletonDefaults.Scope)
				assert.Equal(t, 1, singletonDefaults.MaxInstances, "Singleton should default to 1 instance")

				transientDefaults := GetDefaultScopeConfig(ServiceScopeTransient)
				assert.Equal(t, ServiceScopeTransient, transientDefaults.Scope)
				assert.Greater(t, transientDefaults.MaxInstances, 1, "Transient should allow multiple instances")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}