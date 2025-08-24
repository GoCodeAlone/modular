// Package reverseproxy provides error definitions for the reverse proxy module.
package reverseproxy

import "errors"

// Error definitions for the reverse proxy module.
var (
	// ErrCircuitOpen defined in circuit_breaker.go
	ErrMaxRetriesReached        = errors.New("maximum number of retries reached")
	ErrRequestTimeout           = errors.New("request timed out")
	ErrNoAvailableBackend       = errors.New("no available backend")
	ErrBackendServiceNotFound   = errors.New("backend service not found")
	ErrConfigurationNil         = errors.New("configuration is nil")
	ErrDefaultBackendNotDefined = errors.New("default backend is not defined in backend_services")
	ErrTenantIDRequired         = errors.New("tenant ID is required but TenantIDHeader is not set")
	ErrServiceNotHandleFunc     = errors.New("service does not implement HandleFunc interface")
	ErrCannotRegisterRoutes     = errors.New("cannot register routes: router is nil")
	ErrBackendNotFound          = errors.New("backend not found")
	ErrBackendProxyNil          = errors.New("backend proxy is nil")
	ErrFeatureFlagNotFound      = errors.New("feature flag not found")
	ErrDryRunModeNotEnabled     = errors.New("dry-run mode is not enabled")
	ErrApplicationNil           = errors.New("app cannot be nil")
	ErrLoggerNil                = errors.New("logger cannot be nil")

	// Event observation errors
	ErrNoSubjectForEventEmission = errors.New("no subject available for event emission")
)
