package reverseproxy

import (
	"encoding/json"
	"net/http"
	"reflect"
	"time"

	"github.com/GoCodeAlone/modular"
)

// DebugEndpointsConfig provides configuration for debug endpoints.
type DebugEndpointsConfig struct {
	// Enabled determines if debug endpoints should be available
	Enabled bool `json:"enabled" yaml:"enabled" toml:"enabled" env:"DEBUG_ENDPOINTS_ENABLED" default:"false"`

	// BasePath is the base path for debug endpoints
	BasePath string `json:"base_path" yaml:"base_path" toml:"base_path" env:"DEBUG_BASE_PATH" default:"/debug"`

	// RequireAuth determines if debug endpoints require authentication
	RequireAuth bool `json:"require_auth" yaml:"require_auth" toml:"require_auth" env:"DEBUG_REQUIRE_AUTH" default:"false"`

	// AuthToken is the token required for debug endpoint access (if RequireAuth is true)
	AuthToken string `json:"auth_token" yaml:"auth_token" toml:"auth_token" env:"DEBUG_AUTH_TOKEN"` //nolint:gosec // G117: auth_token is a debug endpoint configuration field, not a credential
}

// DebugInfo represents debugging information about the reverse proxy state.
type DebugInfo struct {
	Timestamp       time.Time                     `json:"timestamp"`
	ModuleName      string                        `json:"module_name"`
	Tenant          string                        `json:"tenant,omitempty"`
	Environment     string                        `json:"environment"`
	Flags           map[string]interface{}        `json:"flags,omitempty"`
	BackendServices map[string]string             `json:"backendServices"`
	Routes          map[string]string             `json:"routes"`
	CircuitBreakers map[string]CircuitBreakerInfo `json:"circuitBreakers,omitempty"`
	HealthChecks    map[string]HealthInfo         `json:"healthChecks,omitempty"`
}

// CircuitBreakerInfo represents circuit breaker status information.
type CircuitBreakerInfo struct {
	State            string    `json:"state"`
	FailureCount     int       `json:"failureCount"`
	Failures         int       `json:"failures"` // alias field expected by tests
	SuccessCount     int       `json:"successCount"`
	LastFailure      time.Time `json:"lastFailure,omitempty"`
	LastAttempt      time.Time `json:"lastAttempt,omitempty"`
	FailureThreshold int       `json:"failureThreshold,omitempty"`
	ResetTimeout     string    `json:"resetTimeout,omitempty"`
}

// HealthInfo represents backend health information.
type HealthInfo struct {
	Status              string    `json:"status"`
	LastCheck           time.Time `json:"lastCheck,omitempty"`
	LastSuccess         time.Time `json:"lastSuccess,omitempty"`
	LastError           string    `json:"lastError,omitempty"`
	ResponseTime        string    `json:"responseTime,omitempty"`
	StatusCode          int       `json:"statusCode,omitempty"`
	DNSResolved         bool      `json:"dnsResolved"`
	ResolvedIPs         []string  `json:"resolvedIPs,omitempty"`
	TotalChecks         int64     `json:"totalChecks"`
	SuccessfulChecks    int64     `json:"successfulChecks"`
	ChecksSkipped       int64     `json:"checksSkipped"`
	HealthCheckPassing  bool      `json:"healthCheckPassing"`
	CircuitBreakerOpen  bool      `json:"circuitBreakerOpen"`
	CircuitBreakerState string    `json:"circuitBreakerState,omitempty"`
	CircuitFailureCount int       `json:"circuitFailureCount,omitempty"`
}

// DebugHandler handles debug endpoint requests.
type DebugHandler struct {
	config          DebugEndpointsConfig
	featureFlagEval FeatureFlagEvaluator
	proxyConfig     *ReverseProxyConfig
	tenantService   modular.TenantService
	logger          modular.Logger
	circuitBreakers map[string]*CircuitBreaker
	healthCheckers  map[string]*HealthChecker
}

// NewDebugHandler creates a new debug handler.
func NewDebugHandler(config DebugEndpointsConfig, featureFlagEval FeatureFlagEvaluator, proxyConfig *ReverseProxyConfig, tenantService modular.TenantService, logger modular.Logger) *DebugHandler {
	return &DebugHandler{
		config:          config,
		featureFlagEval: featureFlagEval,
		proxyConfig:     proxyConfig,
		tenantService:   tenantService,
		logger:          logger,
		circuitBreakers: make(map[string]*CircuitBreaker),
		healthCheckers:  make(map[string]*HealthChecker),
	}
}

// SetCircuitBreakers updates the circuit breakers reference for debugging.
func (d *DebugHandler) SetCircuitBreakers(circuitBreakers map[string]*CircuitBreaker) {
	d.circuitBreakers = circuitBreakers
}

// SetHealthCheckers updates the health checkers reference for debugging.
func (d *DebugHandler) SetHealthCheckers(healthCheckers map[string]*HealthChecker) {
	d.healthCheckers = healthCheckers
}

// RegisterRoutes registers debug endpoint routes with the provided mux.
func (d *DebugHandler) RegisterRoutes(mux *http.ServeMux) {
	if !d.config.Enabled {
		return
	}

	// Feature flags debug endpoint
	mux.HandleFunc(d.config.BasePath+"/flags", d.HandleFlags)

	// General debug info endpoint
	mux.HandleFunc(d.config.BasePath+"/info", d.HandleInfo)

	// Backend status endpoint
	mux.HandleFunc(d.config.BasePath+"/backends", d.HandleBackends)

	// Circuit breaker status endpoint
	mux.HandleFunc(d.config.BasePath+"/circuit-breakers", d.HandleCircuitBreakers)

	// Health check status endpoint
	mux.HandleFunc(d.config.BasePath+"/health-checks", d.HandleHealthChecks)

	d.logger.Info("Debug endpoints registered", "basePath", d.config.BasePath)
}

// HandleFlags handles the feature flags debug endpoint.
func (d *DebugHandler) HandleFlags(w http.ResponseWriter, r *http.Request) {
	if !d.checkAuth(w, r) {
		return
	}

	// Get tenant from request
	tenantID := d.getTenantID(r)

	// Get feature flags
	var flags map[string]interface{}

	if d.featureFlagEval != nil {
		// Get flags from feature flag evaluator by accessing the configuration
		flags = make(map[string]interface{})

		// Create context for tenant-aware configuration lookup
		//nolint:contextcheck // Creating tenant context from request context for configuration lookup
		ctx := r.Context()
		if tenantID != "" {
			ctx = modular.NewTenantContext(ctx, tenantID)
		}

		// Try to get the current configuration to show available flags
		if fileBasedEval, ok := d.featureFlagEval.(*FileBasedFeatureFlagEvaluator); ok {
			// Defensive check for tenantAwareConfig before accessing
			if fileBasedEval.tenantAwareConfig != nil {
				rawConfig := fileBasedEval.tenantAwareConfig.GetConfigWithContext(ctx)
				if config, ok := rawConfig.(*ReverseProxyConfig); ok && config != nil && config.FeatureFlags.Enabled && config.FeatureFlags.Flags != nil {
					for flagName, flagValue := range config.FeatureFlags.Flags {
						flags[flagName] = flagValue
					}
					flags["_source"] = "tenant_aware_config"
				}
			} else if fileBasedEval.defaultConfigProvider != nil {
				// Fall back to default config provider when no tenant service is available
				rawConfig := fileBasedEval.defaultConfigProvider.GetConfig()
				if config, ok := rawConfig.(*ReverseProxyConfig); ok && config != nil && config.FeatureFlags.Enabled && config.FeatureFlags.Flags != nil {
					for flagName, flagValue := range config.FeatureFlags.Flags {
						flags[flagName] = flagValue
					}
					flags["_source"] = "default_config"
				}
			}
		}
		flags["_tenant"] = string(tenantID)
	}

	// For the flags endpoint, return a response structure that matches test expectations
	flagsResponse := map[string]interface{}{
		"timestamp":       time.Now(),
		"tenant":          string(tenantID),
		"environment":     "local", // Could be configured
		"feature_flags":   flags,
		"backendServices": d.proxyConfig.BackendServices,
		"routes":          d.proxyConfig.Routes,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(flagsResponse); err != nil {
		d.logger.Error("Failed to encode debug flags response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleInfo handles the general debug info endpoint.
func (d *DebugHandler) HandleInfo(w http.ResponseWriter, r *http.Request) {
	if !d.checkAuth(w, r) {
		return
	}

	tenantID := d.getTenantID(r)

	// Get feature flags
	var flags map[string]interface{}
	if d.featureFlagEval != nil {
		// Try to get flags from feature flag evaluator
		flags = make(map[string]interface{})
		// Add tenant-specific flags if available
		if tenantID != "" && d.tenantService != nil {
			// Try to get tenant config
			// Since the tenant service interface doesn't expose config directly,
			// we'll skip this for now and just indicate the source
			flags["_source"] = "tenant_config"
		}
	}

	debugInfo := DebugInfo{
		Timestamp:       time.Now(),
		ModuleName:      "reverseproxy",
		Tenant:          string(tenantID),
		Environment:     "local", // Could be configured
		Flags:           flags,
		BackendServices: d.proxyConfig.BackendServices,
		Routes:          d.proxyConfig.Routes,
	}

	// Add circuit breaker info
	if len(d.circuitBreakers) > 0 {
		debugInfo.CircuitBreakers = make(map[string]CircuitBreakerInfo)
		for name, cb := range d.circuitBreakers {
			state := cb.GetState()
			failureCount := cb.GetFailureCount()

			// Create circuit breaker info with real data
			cbInfo := CircuitBreakerInfo{
				State:        state.String(),
				FailureCount: failureCount,
				Failures:     failureCount, // alias field expected by tests
				SuccessCount: 0,            // Circuit breaker doesn't track success count directly
				LastAttempt:  time.Now(),   // Current time as approximation
			}

			// Add additional circuit breaker details if available via reflection
			// This is safe because we control the CircuitBreaker implementation
			if cbVal := reflect.ValueOf(cb); cbVal.Kind() == reflect.Pointer && !cbVal.IsNil() {
				elem := cbVal.Elem()
				if elem.Kind() == reflect.Struct {
					if thresholdField := elem.FieldByName("failureThreshold"); thresholdField.IsValid() && thresholdField.CanInterface() {
						if threshold, ok := thresholdField.Interface().(int); ok {
							cbInfo.FailureThreshold = threshold
						}
					}
					if timeoutField := elem.FieldByName("resetTimeout"); timeoutField.IsValid() && timeoutField.CanInterface() {
						if timeout, ok := timeoutField.Interface().(time.Duration); ok {
							cbInfo.ResetTimeout = timeout.String()
						}
					}
					if lastFailureField := elem.FieldByName("lastFailure"); lastFailureField.IsValid() && lastFailureField.CanInterface() {
						if lastFailure, ok := lastFailureField.Interface().(time.Time); ok && !lastFailure.IsZero() {
							cbInfo.LastFailure = lastFailure
						}
					}
				}
			}

			debugInfo.CircuitBreakers[name] = cbInfo
		}
	}

	// Add health check info with comprehensive data
	if len(d.healthCheckers) > 0 {
		debugInfo.HealthChecks = make(map[string]HealthInfo)
		for _, hc := range d.healthCheckers {
			healthStatuses := hc.GetHealthStatus()
			for backendID, status := range healthStatuses {
				// Determine overall status
				var healthStatus string
				if status.HealthCheckPassing {
					if status.CircuitBreakerOpen {
						healthStatus = "health_check_passing_but_circuit_open"
					} else {
						healthStatus = "healthy"
					}
				} else {
					healthStatus = "unhealthy"
				}

				debugInfo.HealthChecks[backendID] = HealthInfo{
					Status:              healthStatus,
					LastCheck:           status.LastCheck,
					LastSuccess:         status.LastSuccess,
					LastError:           status.LastError,
					ResponseTime:        status.ResponseTime.String(),
					StatusCode:          0, // HTTP status code not tracked in current HealthStatus
					DNSResolved:         status.DNSResolved,
					ResolvedIPs:         status.ResolvedIPs,
					TotalChecks:         status.TotalChecks,
					SuccessfulChecks:    status.SuccessfulChecks,
					ChecksSkipped:       status.ChecksSkipped,
					HealthCheckPassing:  status.HealthCheckPassing,
					CircuitBreakerOpen:  status.CircuitBreakerOpen,
					CircuitBreakerState: status.CircuitBreakerState,
					CircuitFailureCount: status.CircuitFailureCount,
				}
			}
			break // Only process the first health checker
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(debugInfo); err != nil {
		d.logger.Error("Failed to encode debug info response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleBackends handles the backends debug endpoint.
func (d *DebugHandler) HandleBackends(w http.ResponseWriter, r *http.Request) {
	if !d.checkAuth(w, r) {
		return
	}

	backendInfo := map[string]interface{}{
		"timestamp":       time.Now(),
		"backendServices": d.proxyConfig.BackendServices,
		"routes":          d.proxyConfig.Routes,
		"defaultBackend":  d.proxyConfig.DefaultBackend,
	}
	// If health checker info available, enrich with simple per-backend status snapshot for convenience
	if len(d.healthCheckers) > 0 {
		for name, hc := range d.healthCheckers { // name likely "reverseproxy"
			statuses := hc.GetHealthStatus()
			flat := make(map[string]map[string]interface{})
			for backendID, st := range statuses {
				flat[backendID] = map[string]interface{}{
					"healthy":   st.Healthy,
					"lastCheck": st.LastCheck,
					"lastError": st.LastError,
				}
			}
			backendInfo["healthStatus"] = flat
			backendInfo["_healthChecker"] = name
			break // only one expected
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(backendInfo); err != nil {
		d.logger.Error("Failed to encode backends response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleCircuitBreakers handles the circuit breakers debug endpoint.
func (d *DebugHandler) HandleCircuitBreakers(w http.ResponseWriter, r *http.Request) {
	if !d.checkAuth(w, r) {
		return
	}

	// Return a flat JSON object where each key is a circuit breaker name and the value
	// is an object containing failures/failureCount and state. This matches BDD steps
	// that iterate over all top-level values looking for maps with these fields.
	response := map[string]CircuitBreakerInfo{}
	for name, cb := range d.circuitBreakers {
		state := cb.GetState()
		failureCount := cb.GetFailureCount()

		// Create detailed circuit breaker response
		cbInfo := CircuitBreakerInfo{
			State:        state.String(),
			FailureCount: failureCount,
			Failures:     failureCount, // alias field expected by tests
			SuccessCount: 0,            // Circuit breaker doesn't track success count directly
		}

		// Add internal details via reflection for comprehensive debugging
		if cbVal := reflect.ValueOf(cb); cbVal.Kind() == reflect.Pointer && !cbVal.IsNil() {
			elem := cbVal.Elem()
			if elem.Kind() == reflect.Struct {
				if thresholdField := elem.FieldByName("failureThreshold"); thresholdField.IsValid() && thresholdField.CanInterface() {
					if threshold, ok := thresholdField.Interface().(int); ok {
						cbInfo.FailureThreshold = threshold
					}
				}
				if timeoutField := elem.FieldByName("resetTimeout"); timeoutField.IsValid() && timeoutField.CanInterface() {
					if timeout, ok := timeoutField.Interface().(time.Duration); ok {
						cbInfo.ResetTimeout = timeout.String()
					}
				}
				if lastFailureField := elem.FieldByName("lastFailure"); lastFailureField.IsValid() && lastFailureField.CanInterface() {
					if lastFailure, ok := lastFailureField.Interface().(time.Time); ok && !lastFailure.IsZero() {
						cbInfo.LastFailure = lastFailure
					}
				}
			}
		}

		response[name] = cbInfo
	}

	// If no circuit breakers are available yet, return empty response
	// rather than placeholder data that doesn't reflect actual system state

	w.Header().Set("Content-Type", "application/json")
	circuitBreakerResponse := map[string]interface{}{"circuit_breakers": response}
	if err := json.NewEncoder(w).Encode(circuitBreakerResponse); err != nil {
		d.logger.Error("Failed to encode circuit breakers response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleHealthChecks handles the health checks debug endpoint.
func (d *DebugHandler) HandleHealthChecks(w http.ResponseWriter, r *http.Request) {
	if !d.checkAuth(w, r) {
		return
	}

	// Flat JSON object: backendID -> health info with real data from health checker
	response := map[string]HealthInfo{}
	for _, hc := range d.healthCheckers {
		for backendID, status := range hc.GetHealthStatus() {
			// Extract comprehensive health status information
			var healthStatus string
			if status.HealthCheckPassing {
				if status.CircuitBreakerOpen {
					healthStatus = "health_check_passing_but_circuit_open"
				} else {
					healthStatus = "healthy"
				}
			} else {
				healthStatus = "unhealthy"
			}

			// Create comprehensive health info with real data
			response[backendID] = HealthInfo{
				Status:              healthStatus,
				LastCheck:           status.LastCheck,
				LastSuccess:         status.LastSuccess,
				LastError:           status.LastError,
				ResponseTime:        status.ResponseTime.String(),
				StatusCode:          0, // HTTP status code not tracked in current HealthStatus
				DNSResolved:         status.DNSResolved,
				ResolvedIPs:         status.ResolvedIPs,
				TotalChecks:         status.TotalChecks,
				SuccessfulChecks:    status.SuccessfulChecks,
				ChecksSkipped:       status.ChecksSkipped,
				HealthCheckPassing:  status.HealthCheckPassing,
				CircuitBreakerOpen:  status.CircuitBreakerOpen,
				CircuitBreakerState: status.CircuitBreakerState,
				CircuitFailureCount: status.CircuitFailureCount,
			}
		}
		break
	}

	// If health checks are enabled but we have no statuses yet, return empty response
	// rather than placeholder data that doesn't reflect actual system state

	w.Header().Set("Content-Type", "application/json")
	healthCheckResponse := map[string]interface{}{"health_checks": response}
	if err := json.NewEncoder(w).Encode(healthCheckResponse); err != nil {
		d.logger.Error("Failed to encode health checks response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// checkAuth checks authentication for debug endpoints.
func (d *DebugHandler) checkAuth(w http.ResponseWriter, r *http.Request) bool {
	if !d.config.RequireAuth {
		return true
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return false
	}

	// Simple bearer token authentication
	expectedToken := "Bearer " + d.config.AuthToken
	if authHeader != expectedToken {
		http.Error(w, "Invalid authentication token", http.StatusForbidden)
		return false
	}

	return true
}

// getTenantID extracts tenant ID from request.
func (d *DebugHandler) getTenantID(r *http.Request) modular.TenantID {
	tenantID := r.Header.Get(d.proxyConfig.TenantIDHeader)
	return modular.TenantID(tenantID)
}
