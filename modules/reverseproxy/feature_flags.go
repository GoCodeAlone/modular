package reverseproxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"sort"

	"github.com/GoCodeAlone/modular"
)

// FeatureFlagEvaluator defines the interface for evaluating feature flags.
// This allows for different implementations of feature flag services while
// providing a consistent interface for the reverseproxy module.
//
// Evaluators may return special sentinel errors to control aggregation behavior:
//   - ErrNoDecision: Evaluator abstains and evaluation continues to next evaluator
//   - ErrEvaluatorFatal: Fatal error that stops evaluation chain immediately
type FeatureFlagEvaluator interface {
	// EvaluateFlag evaluates a feature flag for the given context and request.
	// Returns true if the feature flag is enabled, false otherwise.
	// The tenantID parameter can be empty if no tenant context is available.
	//
	// Special error handling:
	// - Returning ErrNoDecision allows evaluation to continue to next evaluator
	// - Returning ErrEvaluatorFatal stops evaluation chain immediately
	// - Other errors are treated as non-fatal and evaluation continues
	EvaluateFlag(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request) (bool, error)

	// EvaluateFlagWithDefault evaluates a feature flag with a default value.
	// If evaluation fails or the flag doesn't exist, returns the default value.
	EvaluateFlagWithDefault(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request, defaultValue bool) bool
}

// WeightedEvaluator is an optional interface that FeatureFlagEvaluator implementations
// can implement to specify their priority in the evaluation chain.
// Lower weight values indicate higher priority (evaluated first).
// Default weight for evaluators that don't implement this interface is 100.
// The built-in file evaluator has weight 1000 (lowest priority/last fallback).
type WeightedEvaluator interface {
	FeatureFlagEvaluator
	// Weight returns the priority weight for this evaluator.
	// Lower values = higher priority. Default is 100 if not implemented.
	Weight() int
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

// FeatureFlagAggregator implements FeatureFlagEvaluator by aggregating multiple
// evaluators and calling them in priority order (weight-based).
// It discovers evaluators from the service registry by name prefix pattern.
type FeatureFlagAggregator struct {
	app    modular.Application
	logger *slog.Logger
}

// weightedEvaluatorInstance holds an evaluator with its resolved weight
type weightedEvaluatorInstance struct {
	evaluator FeatureFlagEvaluator
	weight    int
	name      string
}

// NewFeatureFlagAggregator creates a new aggregator that discovers and coordinates
// multiple feature flag evaluators from the application's service registry.
func NewFeatureFlagAggregator(app modular.Application, logger *slog.Logger) *FeatureFlagAggregator {
	return &FeatureFlagAggregator{
		app:    app,
		logger: logger,
	}
}

// discoverEvaluators finds all FeatureFlagEvaluator services by matching interface implementation
// and assigns unique names. The name doesn't matter for matching, only for uniqueness.
func (a *FeatureFlagAggregator) discoverEvaluators() []weightedEvaluatorInstance {
	var evaluators []weightedEvaluatorInstance
	nameCounters := make(map[string]int) // Track name usage for uniqueness

	// Use interface-based discovery to find all FeatureFlagEvaluator services
	evaluatorType := reflect.TypeOf((*FeatureFlagEvaluator)(nil)).Elem()
	entries := a.app.ServiceIntrospector().GetServicesByInterface(evaluatorType)
	for _, entry := range entries {
		// Check if it's the same instance as ourselves (prevent self-ingestion)
		if entry.Service == a {
			continue
		}

		// Skip the aggregator itself to prevent recursion
		if entry.ActualName == "featureFlagEvaluator" {
			continue
		}

		// Skip the internal file evaluator to prevent double evaluation
		// (it will be included via separate discovery)
		if entry.ActualName == "featureFlagEvaluator.file" {
			continue
		}

		// Already confirmed to be FeatureFlagEvaluator by interface discovery
		evaluator := entry.Service.(FeatureFlagEvaluator)

		// Generate unique name using enhanced service registry information
		uniqueName := a.generateUniqueNameWithModuleInfo(entry, nameCounters)

		// Determine weight
		weight := 100 // default weight
		if weightedEvaluator, ok := evaluator.(WeightedEvaluator); ok {
			weight = weightedEvaluator.Weight()
		}

		evaluators = append(evaluators, weightedEvaluatorInstance{
			evaluator: evaluator,
			weight:    weight,
			name:      uniqueName,
		})

		a.logger.Debug("Discovered feature flag evaluator",
			"originalName", entry.OriginalName, "actualName", entry.ActualName,
			"uniqueName", uniqueName, "moduleName", entry.ModuleName,
			"weight", weight, "type", fmt.Sprintf("%T", evaluator))
	}

	// Also include the file evaluator with weight 1000 (lowest priority)
	var fileEvaluator FeatureFlagEvaluator
	if err := a.app.GetService("featureFlagEvaluator.file", &fileEvaluator); err == nil && fileEvaluator != nil {
		evaluators = append(evaluators, weightedEvaluatorInstance{
			evaluator: fileEvaluator,
			weight:    1000, // Lowest priority - fallback evaluator
			name:      "featureFlagEvaluator.file",
		})
	} else if err != nil {
		a.logger.Debug("File evaluator not found", "error", err)
	}

	// Sort by weight (ascending - lower weight = higher priority)
	sort.Slice(evaluators, func(i, j int) bool {
		return evaluators[i].weight < evaluators[j].weight
	})

	return evaluators
}

// generateUniqueNameWithModuleInfo creates a unique name for a feature flag evaluator service
// using the enhanced service registry information that tracks module associations.
// This replaces the previous heuristic-based approach with precise module information.
func (a *FeatureFlagAggregator) generateUniqueNameWithModuleInfo(entry *modular.ServiceRegistryEntry, nameCounters map[string]int) string {
	// Try original name first
	originalName := entry.OriginalName
	if nameCounters[originalName] == 0 {
		nameCounters[originalName] = 1
		return originalName
	}

	// Name conflicts exist - use module information for disambiguation
	if entry.ModuleName != "" {
		// Try with module name
		moduleBasedName := fmt.Sprintf("%s.%s", originalName, entry.ModuleName)
		if nameCounters[moduleBasedName] == 0 {
			nameCounters[moduleBasedName] = 1
			return moduleBasedName
		}
	}

	// Try with module type name if available
	if entry.ModuleType != nil {
		typeName := entry.ModuleType.Elem().Name()
		if typeName == "" {
			typeName = entry.ModuleType.String()
		}
		typeBasedName := fmt.Sprintf("%s.%s", originalName, typeName)
		if nameCounters[typeBasedName] == 0 {
			nameCounters[typeBasedName] = 1
			return typeBasedName
		}
	}

	// Final fallback: append incrementing counter
	counter := nameCounters[originalName]
	nameCounters[originalName] = counter + 1
	return fmt.Sprintf("%s.%d", originalName, counter)
}

// EvaluateFlag implements FeatureFlagEvaluator by calling discovered evaluators
// in weight order until one returns a decision or all have been tried.
func (a *FeatureFlagAggregator) EvaluateFlag(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request) (bool, error) {
	evaluators := a.discoverEvaluators()
	if len(evaluators) == 0 {
		a.logger.Debug("No feature flag evaluators found", "flag", flagID)
		return false, fmt.Errorf("%w: %s", ErrNoEvaluatorsAvailable, flagID)
	}
	for _, eval := range evaluators {
		if eval.evaluator == nil {
			a.logger.Warn("Skipping nil evaluator", "name", eval.name)
			continue
		}
		a.logger.Debug("Trying feature flag evaluator", "evaluator", eval.name, "weight", eval.weight, "flag", flagID)
		result, err := eval.evaluator.EvaluateFlag(ctx, flagID, tenantID, req)
		if err != nil {
			if errors.Is(err, ErrNoDecision) {
				a.logger.Debug("Evaluator abstained", "evaluator", eval.name, "flag", flagID)
				continue
			}
			if errors.Is(err, ErrEvaluatorFatal) {
				a.logger.Error("Evaluator returned fatal error", "evaluator", eval.name, "flag", flagID, "error", err)
				return false, fmt.Errorf("%w: evaluator %s: %w", ErrEvaluatorFatal, eval.name, err)
			}
			a.logger.Warn("Evaluator error (continuing)", "evaluator", eval.name, "flag", flagID, "error", err)
			continue
		}
		a.logger.Debug("Evaluator made decision", "evaluator", eval.name, "flag", flagID, "result", result)
		return result, nil
	}
	a.logger.Debug("No evaluator provided decision", "flag", flagID)
	return false, fmt.Errorf("%w: %s", ErrNoEvaluatorDecision, flagID)
}

// EvaluateFlagWithDefault implements FeatureFlagEvaluator by evaluating a flag
// and returning defaultValue when any error occurs (including no decision).
func (a *FeatureFlagAggregator) EvaluateFlagWithDefault(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request, defaultValue bool) bool {
	val, err := a.EvaluateFlag(ctx, flagID, tenantID, req)
	if err != nil {
		return defaultValue
	}
	return val
}
