package reverseproxy

import (
	"context"
	"net/http"

	"github.com/CrisisTextLine/modular"
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

// FileBasedFeatureFlagEvaluator implements a simple file-based feature flag evaluator.
// This is primarily intended for testing and examples.
type FileBasedFeatureFlagEvaluator struct {
	// flags maps feature flag IDs to their enabled state
	flags map[string]bool

	// tenantFlags maps tenant IDs to their specific feature flag overrides
	tenantFlags map[modular.TenantID]map[string]bool
}

// NewFileBasedFeatureFlagEvaluator creates a new file-based feature flag evaluator.
func NewFileBasedFeatureFlagEvaluator() *FileBasedFeatureFlagEvaluator {
	return &FileBasedFeatureFlagEvaluator{
		flags:       make(map[string]bool),
		tenantFlags: make(map[modular.TenantID]map[string]bool),
	}
}

// SetFlag sets a global feature flag value.
func (f *FileBasedFeatureFlagEvaluator) SetFlag(flagID string, enabled bool) {
	f.flags[flagID] = enabled
}

// SetTenantFlag sets a tenant-specific feature flag value.
func (f *FileBasedFeatureFlagEvaluator) SetTenantFlag(tenantID modular.TenantID, flagID string, enabled bool) {
	if f.tenantFlags[tenantID] == nil {
		f.tenantFlags[tenantID] = make(map[string]bool)
	}
	f.tenantFlags[tenantID][flagID] = enabled
}

// EvaluateFlag evaluates a feature flag for the given context and request.
func (f *FileBasedFeatureFlagEvaluator) EvaluateFlag(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request) (bool, error) {
	// Check tenant-specific flags first
	if tenantID != "" {
		if tenantFlagMap, exists := f.tenantFlags[tenantID]; exists {
			if value, exists := tenantFlagMap[flagID]; exists {
				return value, nil
			}
		}
	}

	// Fall back to global flags
	if value, exists := f.flags[flagID]; exists {
		return value, nil
	}

	// Flag not found, return error to indicate flag doesn't exist
	return false, ErrFeatureFlagNotFound
}

// EvaluateFlagWithDefault evaluates a feature flag with a default value.
func (f *FileBasedFeatureFlagEvaluator) EvaluateFlagWithDefault(ctx context.Context, flagID string, tenantID modular.TenantID, req *http.Request, defaultValue bool) bool {
	value, err := f.EvaluateFlag(ctx, flagID, tenantID, req)
	if err != nil {
		return defaultValue
	}
	return value
}
