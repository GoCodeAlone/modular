// Package reverseproxy provides error definitions for the reverse proxy module.
package reverseproxy

import "errors"

// Error definitions for the reverse proxy module.
var (
	// ErrCircuitOpen defined in circuit_breaker.go
	ErrMaxRetriesReached               = errors.New("maximum number of retries reached")
	ErrRequestTimeout                  = errors.New("request timed out")
	ErrNoAvailableBackend              = errors.New("no available backend")
	ErrBackendServiceNotFound          = errors.New("backend service not found")
	ErrConfigurationNil                = errors.New("configuration is nil")
	ErrDefaultBackendNotDefined        = errors.New("default backend is not defined in backend_services")
	ErrTenantIDRequired                = errors.New("tenant ID is required but TenantIDHeader is not set")
	ErrServiceNotHandleFunc            = errors.New("service does not implement HandleFunc interface")
	ErrCannotRegisterRoutes            = errors.New("cannot register routes: router is nil")
	ErrBackendNotFound                 = errors.New("backend not found")
	ErrBackendProxyNil                 = errors.New("backend proxy is nil")
	ErrFeatureFlagNotFound             = errors.New("feature flag not found")
	ErrDryRunModeNotEnabled            = errors.New("dry-run mode is not enabled")
	ErrApplicationNil                  = errors.New("app cannot be nil")
	ErrLoggerNil                       = errors.New("logger cannot be nil")
	ErrTenantAwareConfigCreation       = errors.New("failed to create tenant-aware config for feature flags")
	ErrInvalidFeatureFlagConfigType    = errors.New("invalid feature flag configuration type")
	ErrNoFeatureFlagConfigProvider     = errors.New("no configuration provider available for feature flags")
	ErrInvalidDefaultFeatureFlagConfig = errors.New("invalid default configuration type for feature flags")
	ErrConfigurationNotLoaded          = errors.New("configuration not loaded")
	ErrBackendErrorStatus              = errors.New("backend returned non-success status")

	// Feature flag evaluation sentinel errors
	ErrNoDecision     = errors.New("no-decision")     // Evaluator abstains from making a decision
	ErrEvaluatorFatal = errors.New("evaluator-fatal") // Fatal error that should abort evaluation chain

	// Feature flag aggregator errors
	ErrNoEvaluatorsAvailable = errors.New("no feature flag evaluators available")
	ErrNoEvaluatorDecision   = errors.New("no evaluator provided decision for flag")

	// Event observation errors
	ErrNoSubjectForEventEmission = errors.New("no subject available for event emission")

	// Dynamic operation errors
	ErrBackendIDRequired          = errors.New("backend id required")
	ErrServiceURLRequired         = errors.New("service URL required")
	ErrNoBackendsConfigured       = errors.New("no backends configured")
	ErrBackendNotConfigured       = errors.New("backend not configured")
	ErrInvalidEmptyResponsePolicy = errors.New("invalid empty_policy: must be one of allow-empty, skip-empty, fail-on-empty")
)
