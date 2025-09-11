package modular

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithTenantGuardModeOption(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_define_with_tenant_guard_mode_option_function",
			testFunc: func(t *testing.T) {
				// Test that WithTenantGuardMode function exists
				option := WithTenantGuardMode(TenantGuardModeStrict)
				assert.NotNil(t, option, "WithTenantGuardMode should return an application option")
			},
		},
		{
			name: "should_accept_tenant_guard_mode_configuration",
			testFunc: func(t *testing.T) {
				// Test that WithTenantGuardMode accepts different guard modes
				strictOption := WithTenantGuardMode(TenantGuardModeStrict)
				assert.NotNil(t, strictOption, "Should create option with strict mode")

				lenientOption := WithTenantGuardMode(TenantGuardModeLenient)
				assert.NotNil(t, lenientOption, "Should create option with lenient mode")

				disabledOption := WithTenantGuardMode(TenantGuardModeDisabled)
				assert.NotNil(t, disabledOption, "Should create option with disabled mode")
			},
		},
		{
			name: "should_accept_detailed_tenant_guard_configuration",
			testFunc: func(t *testing.T) {
				// Test that WithTenantGuardMode accepts detailed configuration
				config := TenantGuardConfig{
					Mode:               TenantGuardModeStrict,
					EnforceIsolation:   true,
					AllowCrossTenant:   false,
					ValidationTimeout:  5 * time.Second,
					MaxTenantCacheSize: 1000,
					TenantTTL:          10 * time.Minute,
				}

				option := WithTenantGuardModeConfig(config)
				assert.NotNil(t, option, "WithTenantGuardModeConfig should accept detailed configuration")
			},
		},
		{
			name: "should_apply_option_to_application_builder",
			testFunc: func(t *testing.T) {
				// Test that WithTenantGuardMode option can be applied to application builder
				builder := NewApplicationBuilder()
				option := WithTenantGuardMode(TenantGuardModeStrict)
				// builder.WithOption never returns error directly; ensure chain works
				_ = builder.WithOption(option)
				// Build to ensure no panic or error on registration (need logger)
				builder.WithOption(WithLogger(NewTestLogger()))
				_, buildErr := builder.Build()
				assert.NoError(t, buildErr, "Should build with tenant guard option")
			},
		},
		{
			name: "should_configure_tenant_isolation_in_application",
			testFunc: func(t *testing.T) {
				// Test that application built with WithTenantGuardMode enforces tenant isolation
				builder := NewApplicationBuilder()

				app, err := builder.
					WithOption(WithLogger(NewTestLogger())).
					WithOption(WithTenantGuardMode(TenantGuardModeStrict)).
					Build()
				assert.NoError(t, err, "Should build application with tenant guard mode (strict)")

				// Check that application has tenant guard capability
				tenantGuard := app.GetTenantGuard()
				assert.NotNil(t, tenantGuard, "Application should have tenant guard")
				assert.Equal(t, TenantGuardModeStrict, tenantGuard.GetMode(), "Tenant guard should be in strict mode")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestTenantGuardMode(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_define_tenant_guard_mode_constants",
			testFunc: func(t *testing.T) {
				// Test that TenantGuardMode constants are defined
				assert.Equal(t, "strict", string(TenantGuardModeStrict), "TenantGuardModeStrict should be 'strict'")
				assert.Equal(t, "lenient", string(TenantGuardModeLenient), "TenantGuardModeLenient should be 'lenient'")
				assert.Equal(t, "disabled", string(TenantGuardModeDisabled), "TenantGuardModeDisabled should be 'disabled'")
			},
		},
		{
			name: "should_support_string_conversion",
			testFunc: func(t *testing.T) {
				// Test that TenantGuardMode can be converted to string
				mode := TenantGuardModeStrict
				str := mode.String()
				assert.Equal(t, "strict", str, "TenantGuardMode should convert to string")
			},
		},
		{
			name: "should_parse_from_string",
			testFunc: func(t *testing.T) {
				// Test that TenantGuardMode can be parsed from string
				mode, err := ParseTenantGuardMode("strict")
				assert.NoError(t, err, "Should parse valid guard mode")
				assert.Equal(t, TenantGuardModeStrict, mode, "Should parse strict correctly")

				mode, err = ParseTenantGuardMode("lenient")
				assert.NoError(t, err, "Should parse lenient correctly")
				assert.Equal(t, TenantGuardModeLenient, mode)

				mode, err = ParseTenantGuardMode("disabled")
				assert.NoError(t, err, "Should parse disabled correctly")
				assert.Equal(t, TenantGuardModeDisabled, mode)

				_, err = ParseTenantGuardMode("invalid")
				assert.Error(t, err, "Should return error for invalid mode")
			},
		},
		{
			name: "should_determine_enforcement_level",
			testFunc: func(t *testing.T) {
				// Test that guard modes have associated enforcement levels
				assert.True(t, TenantGuardModeStrict.IsEnforcing(), "Strict mode should be enforcing")
				assert.True(t, TenantGuardModeLenient.IsEnforcing(), "Lenient mode should be enforcing")
				assert.False(t, TenantGuardModeDisabled.IsEnforcing(), "Disabled mode should not be enforcing")

				assert.True(t, TenantGuardModeStrict.IsStrict(), "Strict mode should be strict")
				assert.False(t, TenantGuardModeLenient.IsStrict(), "Lenient mode should not be strict")
				assert.False(t, TenantGuardModeDisabled.IsStrict(), "Disabled mode should not be strict")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestTenantGuardConfig(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_define_tenant_guard_config_type",
			testFunc: func(t *testing.T) {
				// Test that TenantGuardConfig type exists with all required fields
				config := TenantGuardConfig{
					Mode:               TenantGuardModeStrict,
					EnforceIsolation:   true,
					AllowCrossTenant:   false,
					ValidationTimeout:  5 * time.Second,
					MaxTenantCacheSize: 1000,
					TenantTTL:          10 * time.Minute,
					LogViolations:      true,
					BlockViolations:    true,
				}

				assert.Equal(t, TenantGuardModeStrict, config.Mode, "TenantGuardConfig should have Mode field")
				assert.True(t, config.EnforceIsolation, "TenantGuardConfig should have EnforceIsolation field")
				assert.False(t, config.AllowCrossTenant, "TenantGuardConfig should have AllowCrossTenant field")
				assert.Equal(t, 5*time.Second, config.ValidationTimeout, "TenantGuardConfig should have ValidationTimeout field")
				assert.Equal(t, 1000, config.MaxTenantCacheSize, "TenantGuardConfig should have MaxTenantCacheSize field")
				assert.Equal(t, 10*time.Minute, config.TenantTTL, "TenantGuardConfig should have TenantTTL field")
				assert.True(t, config.LogViolations, "TenantGuardConfig should have LogViolations field")
				assert.True(t, config.BlockViolations, "TenantGuardConfig should have BlockViolations field")
			},
		},
		{
			name: "should_validate_tenant_guard_config",
			testFunc: func(t *testing.T) {
				// Test config validation
				validConfig := TenantGuardConfig{
					Mode:               TenantGuardModeStrict,
					ValidationTimeout:  5 * time.Second,
					MaxTenantCacheSize: 1000,
					TenantTTL:          10 * time.Minute,
				}
				assert.True(t, validConfig.IsValid(), "Valid config should pass validation")

				invalidConfig := TenantGuardConfig{
					Mode:               TenantGuardModeStrict,
					ValidationTimeout:  -1 * time.Second, // Invalid timeout
					MaxTenantCacheSize: -1,               // Invalid cache size
					TenantTTL:          0,                // Invalid TTL
				}
				assert.False(t, invalidConfig.IsValid(), "Invalid config should fail validation")
			},
		},
		{
			name: "should_provide_default_tenant_guard_config",
			testFunc: func(t *testing.T) {
				// Test default configuration for each mode
				strictDefault := NewDefaultTenantGuardConfig(TenantGuardModeStrict)
				assert.Equal(t, TenantGuardModeStrict, strictDefault.Mode)
				assert.True(t, strictDefault.EnforceIsolation, "Strict mode should enforce isolation by default")
				assert.False(t, strictDefault.AllowCrossTenant, "Strict mode should not allow cross-tenant by default")
				assert.True(t, strictDefault.BlockViolations, "Strict mode should block violations by default")

				lenientDefault := NewDefaultTenantGuardConfig(TenantGuardModeLenient)
				assert.Equal(t, TenantGuardModeLenient, lenientDefault.Mode)
				assert.True(t, lenientDefault.LogViolations, "Lenient mode should log violations by default")
				assert.False(t, lenientDefault.BlockViolations, "Lenient mode should not block violations by default")

				disabledDefault := NewDefaultTenantGuardConfig(TenantGuardModeDisabled)
				assert.Equal(t, TenantGuardModeDisabled, disabledDefault.Mode)
				assert.False(t, disabledDefault.EnforceIsolation, "Disabled mode should not enforce isolation")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestTenantGuardBehavior(t *testing.T) {
	tests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T)
	}{
		{
			name:        "should_allow_disabled_mode_early",
			description: "Disabled mode should always allow access without recording violations",
			testFunc: func(t *testing.T) {
				builder := NewApplicationBuilder()
				app, err := builder.
					WithOption(WithLogger(NewTestLogger())).
					WithOption(WithTenantGuardMode(TenantGuardModeDisabled)).
					Build()
				assert.NoError(t, err)

				tg := app.GetTenantGuard()
				if tg == nil {
					return // acceptable if not registered for disabled
				}
				allowed, err := tg.ValidateAccess(context.Background(), &TenantViolation{ViolationType: TenantViolationCrossTenantAccess})
				assert.NoError(t, err)
				assert.True(t, allowed, "Disabled mode must allow access")
				assert.Len(t, tg.GetRecentViolations(), 0, "Disabled mode should not record violations")
			},
		},
		{
			name:        "should_enforce_strict_tenant_isolation",
			description: "Strict tenant guard mode should prevent cross-tenant access",
			testFunc: func(t *testing.T) {
				builder := NewApplicationBuilder()
				config := TenantGuardConfig{
					Mode:             TenantGuardModeStrict,
					EnforceIsolation: true,
					AllowCrossTenant: false,
					BlockViolations:  true,
				}

				app, err := builder.
					WithOption(WithLogger(NewTestLogger())).
					WithOption(WithTenantGuardModeConfig(config)).
					Build()
				require.NoError(t, err, "Should build application with strict tenant guard")

				tenantGuard := app.GetTenantGuard()
				require.NotNil(t, tenantGuard, "Should have tenant guard")

				// Test that cross-tenant access is blocked
				ctx := context.Background()
				ctx = WithTenantContext(ctx, "tenant-a")

				violation := &TenantViolation{
					RequestingTenant: "tenant-a",
					AccessedResource: "tenant-b/resource",
					ViolationType:    TenantViolationCrossTenantAccess,
				}

				allowed, err := tenantGuard.ValidateAccess(ctx, violation)
				assert.NoError(t, err, "Validation should succeed")
				assert.False(t, allowed, "Cross-tenant access should be blocked in strict mode")
			},
		},
		{
			name:        "should_allow_lenient_tenant_access_with_logging",
			description: "Lenient tenant guard mode should allow cross-tenant access but log violations",
			testFunc: func(t *testing.T) {
				builder := NewApplicationBuilder()
				config := TenantGuardConfig{
					Mode:            TenantGuardModeLenient,
					LogViolations:   true,
					BlockViolations: false,
				}

				app, err := builder.
					WithOption(WithLogger(NewTestLogger())).
					WithOption(WithTenantGuardModeConfig(config)).
					Build()
				require.NoError(t, err, "Should build application with lenient tenant guard")

				tenantGuard := app.GetTenantGuard()
				require.NotNil(t, tenantGuard, "Should have tenant guard")

				// Test that cross-tenant access is allowed but logged
				ctx := context.Background()
				ctx = WithTenantContext(ctx, "tenant-a")

				violation := &TenantViolation{
					RequestingTenant: "tenant-a",
					AccessedResource: "tenant-b/resource",
					ViolationType:    TenantViolationCrossTenantAccess,
				}

				allowed, err := tenantGuard.ValidateAccess(ctx, violation)
				assert.NoError(t, err, "Validation should succeed")
				assert.True(t, allowed, "Cross-tenant access should be allowed in lenient mode")

				// Verify violation was logged (would check logs in real implementation)
				violations := tenantGuard.GetRecentViolations()
				assert.Len(t, violations, 1, "Should have recorded the violation")
			},
		},
		{
			name:        "should_disable_tenant_guard_when_disabled_mode",
			description: "Disabled tenant guard mode should not enforce any tenant isolation",
			testFunc: func(t *testing.T) {
				builder := NewApplicationBuilder()

				app, err := builder.
					WithOption(WithLogger(NewTestLogger())).
					WithOption(WithTenantGuardMode(TenantGuardModeDisabled)).
					Build()
				require.NoError(t, err, "Should build application with disabled tenant guard")

				tenantGuard := app.GetTenantGuard()

				// In disabled mode, tenant guard might not exist or be a no-op
				if tenantGuard != nil {
					assert.False(t, tenantGuard.GetMode().IsEnforcing(), "Disabled mode should not be enforcing")

					// All access should be allowed without logging
					ctx := context.Background()
					violation := &TenantViolation{
						RequestingTenant: "tenant-a",
						AccessedResource: "tenant-b/resource",
						ViolationType:    TenantViolationCrossTenantAccess,
					}

					allowed, err := tenantGuard.ValidateAccess(ctx, violation)
					assert.NoError(t, err, "Validation should succeed")
					assert.True(t, allowed, "All access should be allowed in disabled mode")
				}
			},
		},
		{
			name:        "should_support_tenant_whitelisting",
			description: "Tenant guard should support whitelisting specific cross-tenant relationships",
			testFunc: func(t *testing.T) {
				config := TenantGuardConfig{
					Mode:             TenantGuardModeStrict,
					AllowCrossTenant: false,
					CrossTenantWhitelist: map[string][]string{
						"tenant-a": {"tenant-b", "tenant-c"}, // tenant-a can access tenant-b and tenant-c
						"tenant-b": {"tenant-a"},             // tenant-b can access tenant-a
					},
				}

				builder := NewApplicationBuilder()
				app, err := builder.
					WithOption(WithLogger(NewTestLogger())).
					WithOption(WithTenantGuardModeConfig(config)).
					Build()
				require.NoError(t, err, "Should build application with whitelisted cross-tenant access")

				tenantGuard := app.GetTenantGuard()
				require.NotNil(t, tenantGuard, "Should have tenant guard")

				// Test whitelisted access
				ctx := WithTenantContext(context.Background(), "tenant-a")
				violation := &TenantViolation{
					RequestingTenant: "tenant-a",
					AccessedResource: "tenant-b/resource", // whitelisted
					ViolationType:    TenantViolationCrossTenantAccess,
				}

				allowed, err := tenantGuard.ValidateAccess(ctx, violation)
				assert.NoError(t, err, "Validation should succeed")
				assert.True(t, allowed, "Whitelisted cross-tenant access should be allowed")

				// Test non-whitelisted access
				violation.AccessedResource = "tenant-d/resource" // not whitelisted
				allowed, err = tenantGuard.ValidateAccess(ctx, violation)
				assert.NoError(t, err, "Validation should succeed")
				assert.False(t, allowed, "Non-whitelisted cross-tenant access should be blocked")
			},
		},
		{
			name:        "should_handle_unknown_mode_defensive_branch",
			description: "Defensive error path for unknown mode returns error and blocks",
			testFunc: func(t *testing.T) {
				// Create a guard with an invalid mode manually (bypass option validation)
				guard := &stdTenantGuard{config: TenantGuardConfig{Mode: TenantGuardMode("weird")}}
				allowed, err := guard.ValidateAccess(context.Background(), &TenantViolation{ViolationType: TenantViolationCrossTenantAccess})
				assert.Error(t, err)
				assert.False(t, allowed)
			},
		},
		{
			name:        "should_not_panic_on_nil_whitelist",
			description: "Nil whitelist map path returns false (not whitelisted)",
			testFunc: func(t *testing.T) {
				guard := &stdTenantGuard{config: TenantGuardConfig{Mode: TenantGuardModeStrict}}
				allowed, err := guard.ValidateAccess(context.Background(), &TenantViolation{
					ViolationType:    TenantViolationCrossTenantAccess,
					RequestingTenant: "t1",
					AccessedResource: "t2/resource",
				})
				assert.NoError(t, err)
				assert.False(t, allowed)
			},
		},
		{
			name:        "should_respect_whitelist_prefix_exact_boundary",
			description: "Whitelist should match only proper tenant prefix + '/'",
			testFunc: func(t *testing.T) {
				guard := &stdTenantGuard{config: TenantGuardConfig{
					Mode: TenantGuardModeStrict,
					CrossTenantWhitelist: map[string][]string{
						"team": {"tenant"},
					},
				}}
				// resource starts with 'tenantX' not exact 'tenant/' prefix
				allowed, _ := guard.ValidateAccess(context.Background(), &TenantViolation{
					ViolationType:    TenantViolationCrossTenantAccess,
					RequestingTenant: "team",
					AccessedResource: "tenantX/resource",
				})
				assert.False(t, allowed, "Should not allow partial prefix (tenantX)")

				// Proper exact prefix match
				allowed, _ = guard.ValidateAccess(context.Background(), &TenantViolation{
					ViolationType:    TenantViolationCrossTenantAccess,
					RequestingTenant: "team",
					AccessedResource: "tenant/service",
				})
				assert.True(t, allowed, "Should allow exact whitelisted tenant prefix")
			},
		},
		{
			name:        "should_reject_invalid_config_in_option",
			description: "Option should return error on invalid negative values and builder ignores it",
			testFunc: func(t *testing.T) {
				invalid := TenantGuardConfig{Mode: TenantGuardModeStrict, ValidationTimeout: -1}
				op := WithTenantGuardModeConfig(invalid)
				b := NewApplicationBuilder().WithOption(WithLogger(NewTestLogger()))
				b.WithOption(op) // builder ignores internal error but guard should not be registered
				app, err := b.Build()
				assert.NoError(t, err)
				assert.Nil(t, app.GetTenantGuard(), "Invalid config must not register tenant guard")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestTenantViolation(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_define_tenant_violation_type",
			testFunc: func(t *testing.T) {
				// Test that TenantViolation type exists with required fields
				violation := TenantViolation{
					RequestingTenant: "tenant-a",
					AccessedResource: "tenant-b/sensitive-data",
					ViolationType:    TenantViolationCrossTenantAccess,
					Timestamp:        time.Now(),
					Severity:         TenantViolationSeverityHigh,
					Context:          map[string]interface{}{"user_id": "user-123"},
				}

				assert.Equal(t, "tenant-a", violation.RequestingTenant, "TenantViolation should have RequestingTenant field")
				assert.Equal(t, "tenant-b/sensitive-data", violation.AccessedResource, "TenantViolation should have AccessedResource field")
				assert.Equal(t, TenantViolationCrossTenantAccess, violation.ViolationType, "TenantViolation should have ViolationType field")
				assert.NotNil(t, violation.Timestamp, "TenantViolation should have Timestamp field")
				assert.Equal(t, TenantViolationSeverityHigh, violation.Severity, "TenantViolation should have Severity field")
				assert.NotNil(t, violation.Context, "TenantViolation should have Context field")
			},
		},
		{
			name: "should_define_tenant_violation_types",
			testFunc: func(t *testing.T) {
				// Test that TenantViolationType constants are defined
				assert.Equal(t, "cross_tenant_access", string(TenantViolationCrossTenantAccess))
				assert.Equal(t, "invalid_tenant_context", string(TenantViolationInvalidTenantContext))
				assert.Equal(t, "missing_tenant_context", string(TenantViolationMissingTenantContext))
				assert.Equal(t, "unauthorized_tenant_operation", string(TenantViolationUnauthorizedOperation))
			},
		},
		{
			name: "should_define_tenant_violation_severities",
			testFunc: func(t *testing.T) {
				// Test that TenantViolationSeverity constants are defined
				assert.Equal(t, "low", string(TenantViolationSeverityLow))
				assert.Equal(t, "medium", string(TenantViolationSeverityMedium))
				assert.Equal(t, "high", string(TenantViolationSeverityHigh))
				assert.Equal(t, "critical", string(TenantViolationSeverityCritical))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}
