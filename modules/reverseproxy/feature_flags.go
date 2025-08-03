package reverseproxy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/GoCodeAlone/modular"
)

// FeatureFlagEvaluator defines the interface for evaluating feature flags.
// This allows for different implementations of feature flag services while
// providing a consistent interface for the reverseproxy module.
type FeatureFlagEvaluator interface {
	// EvaluateFlag evaluates a feature flag for the given context and request.
	// Returns true if the feature flag is enabled, false otherwise.
	// The tenantID parameter can be empty if no tenant context is available.
	EvaluateFlag(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request) (bool, error)

	// EvaluateFlagWithDefault evaluates a feature flag with a default value.
	// If evaluation fails or the flag doesn't exist, returns the default value.
	EvaluateFlagWithDefault(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request, defaultValue bool) bool
}

// FileBasedFeatureFlagEvaluator implements a feature flag evaluator that integrates
// with the Modular framework's tenant-aware configuration system.
type FileBasedFeatureFlagEvaluator struct {
	// app provides access to the application and its services
	app modular.Application

	// tenantAwareConfig provides tenant-aware access to feature flag configuration
	tenantAwareConfig *modular.TenantAwareConfig

	// logger for debug and error logging
	logger *slog.Logger
}

// NewFileBasedFeatureFlagEvaluator creates a new tenant-aware feature flag evaluator.
func NewFileBasedFeatureFlagEvaluator(app modular.Application, logger *slog.Logger) (*FileBasedFeatureFlagEvaluator, error) {
	// Validate parameters
	if app == nil {
		return nil, ErrApplicationNil
	}
	if logger == nil {
		return nil, ErrLoggerNil
	}
	// Get tenant service
	var tenantService modular.TenantService
	if err := app.GetService("tenantService", &tenantService); err != nil {
		logger.WarnContext(context.Background(), "TenantService not available, feature flags will use default configuration only", "error", err)
		tenantService = nil
	}

	// Get the default configuration from the application
	var defaultConfigProvider modular.ConfigProvider
	if configProvider, err := app.GetConfigSection("reverseproxy"); err == nil {
		defaultConfigProvider = configProvider
	} else {
		// Fallback to empty config if no section is registered
		defaultConfigProvider = modular.NewStdConfigProvider(&ReverseProxyConfig{})
	}

	// Create tenant-aware config for feature flags
	// This will use the "reverseproxy" section from configurations
	tenantAwareConfig := modular.NewTenantAwareConfig(
		defaultConfigProvider,
		tenantService,
		"reverseproxy",
	)

	return &FileBasedFeatureFlagEvaluator{
		app:               app,
		tenantAwareConfig: tenantAwareConfig,
		logger:            logger,
	}, nil
}

// EvaluateFlag evaluates a feature flag using tenant-aware configuration.
// It follows the standard Modular framework pattern where:
// 1. Default flags come from the main configuration
// 2. Tenant-specific overrides come from tenant configuration files
// 3. During request processing, tenant context determines which configuration to use
//
//nolint:contextcheck // Skipping context check because this code intentionally creates a new tenant context if one does not exist, enabling tenant-aware configuration lookup.
func (f *FileBasedFeatureFlagEvaluator) EvaluateFlag(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request) (bool, error) {
	// Create context with tenant ID if provided and not already a tenant context
	if tenantID != "" {
		if _, hasTenant := modular.GetTenantIDFromContext(ctx); !hasTenant {
			ctx = modular.NewTenantContext(ctx, tenantID)
		}
	}

	// Get tenant-aware configuration
	config := f.tenantAwareConfig.GetConfigWithContext(ctx).(*ReverseProxyConfig)
	if config == nil {
		f.logger.DebugContext(ctx, "No feature flag configuration available", "flag", flagID)
		return false, fmt.Errorf("feature flag %s not found: %w", flagID, ErrFeatureFlagNotFound)
	}

	// Check if feature flags are enabled
	if !config.FeatureFlags.Enabled {
		f.logger.DebugContext(ctx, "Feature flags are disabled", "flag", flagID)
		return false, fmt.Errorf("feature flags disabled: %w", ErrFeatureFlagNotFound)
	}

	// Look up the flag value
	if config.FeatureFlags.Flags != nil {
		if value, exists := config.FeatureFlags.Flags[flagID]; exists {
			f.logger.DebugContext(ctx, "Feature flag evaluated",
				"flag", flagID,
				"tenant", tenantID,
				"value", value)
			return value, nil
		}
	}

	f.logger.DebugContext(ctx, "Feature flag not found in configuration",
		"flag", flagID,
		"tenant", tenantID)
	return false, fmt.Errorf("feature flag %s not found: %w", flagID, ErrFeatureFlagNotFound)
}

// EvaluateFlagWithDefault evaluates a feature flag with a default value.
func (f *FileBasedFeatureFlagEvaluator) EvaluateFlagWithDefault(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request, defaultValue bool) bool {
	value, err := f.EvaluateFlag(ctx, flagID, tenantID, req)
	if err != nil {
		return defaultValue
	}
	return value
}
