package reverseproxy

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	AuthToken string `json:"auth_token" yaml:"auth_token" toml:"auth_token" env:"DEBUG_AUTH_TOKEN"`
}

// DebugInfo represents debugging information about the reverse proxy state.
type DebugInfo struct {
	Timestamp       time.Time                     `json:"timestamp"`
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
	State        string    `json:"state"`
	FailureCount int       `json:"failureCount"`
	SuccessCount int       `json:"successCount"`
	LastFailure  time.Time `json:"lastFailure,omitempty"`
	LastAttempt  time.Time `json:"lastAttempt,omitempty"`
}

// HealthInfo represents backend health information.
type HealthInfo struct {
	Status       string    `json:"status"`
	LastCheck    time.Time `json:"lastCheck,omitempty"`
	ResponseTime string    `json:"responseTime,omitempty"`
	StatusCode   int       `json:"statusCode,omitempty"`
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
			config := fileBasedEval.tenantAwareConfig.GetConfigWithContext(ctx).(*ReverseProxyConfig)
			if config != nil && config.FeatureFlags.Enabled && config.FeatureFlags.Flags != nil {
				for flagName, flagValue := range config.FeatureFlags.Flags {
					flags[flagName] = flagValue
				}
				flags["_source"] = "tenant_aware_config"
				flags["_tenant"] = string(tenantID)
			}
		}
	}

	debugInfo := DebugInfo{
		Timestamp:       time.Now(),
		Tenant:          string(tenantID),
		Environment:     "local", // Could be configured
		Flags:           flags,
		BackendServices: d.proxyConfig.BackendServices,
		Routes:          d.proxyConfig.Routes,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(debugInfo); err != nil {
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
			debugInfo.CircuitBreakers[name] = CircuitBreakerInfo{
				State:        cb.GetState().String(),
				FailureCount: 0, // Circuit breaker doesn't expose failure count
				SuccessCount: 0, // Circuit breaker doesn't expose success count
			}
		}
	}

	// Add health check info
	if len(d.healthCheckers) > 0 {
		debugInfo.HealthChecks = make(map[string]HealthInfo)
		for name, hc := range d.healthCheckers {
			healthStatuses := hc.GetHealthStatus()
			if status, exists := healthStatuses[name]; exists {
				debugInfo.HealthChecks[name] = HealthInfo{
					Status:       fmt.Sprintf("healthy=%v", status.Healthy),
					LastCheck:    status.LastCheck,
					ResponseTime: status.ResponseTime.String(),
					StatusCode:   0, // HealthStatus doesn't expose status code directly
				}
			}
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

	cbInfo := make(map[string]CircuitBreakerInfo)

	for name, cb := range d.circuitBreakers {
		cbInfo[name] = CircuitBreakerInfo{
			State:        cb.GetState().String(),
			FailureCount: 0, // Circuit breaker doesn't expose failure count
			SuccessCount: 0, // Circuit breaker doesn't expose success count
		}
	}

	response := map[string]interface{}{
		"timestamp":       time.Now(),
		"circuitBreakers": cbInfo,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		d.logger.Error("Failed to encode circuit breakers response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleHealthChecks handles the health checks debug endpoint.
func (d *DebugHandler) HandleHealthChecks(w http.ResponseWriter, r *http.Request) {
	if !d.checkAuth(w, r) {
		return
	}

	healthInfo := make(map[string]HealthInfo)

	for name, hc := range d.healthCheckers {
		healthStatuses := hc.GetHealthStatus()
		if status, exists := healthStatuses[name]; exists {
			healthInfo[name] = HealthInfo{
				Status:       fmt.Sprintf("healthy=%v", status.Healthy),
				LastCheck:    status.LastCheck,
				ResponseTime: status.ResponseTime.String(),
				StatusCode:   0, // HealthStatus doesn't expose status code directly
			}
		}
	}

	response := map[string]interface{}{
		"timestamp":    time.Now(),
		"healthChecks": healthInfo,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
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
