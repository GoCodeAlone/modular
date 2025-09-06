package reverseproxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"
)

// ErrNoHostname is returned when a URL has no hostname
var ErrNoHostname = errors.New("no hostname in URL")

// ErrUnexpectedStatusCode is returned when a health check receives an unexpected status code
var ErrUnexpectedStatusCode = errors.New("unexpected status code")

// ErrUnexpectedConfigType is returned when an unexpected config type is passed to Init
var ErrUnexpectedConfigType = errors.New("unexpected config type")

// HealthStatus represents the health status of a backend service.
type HealthStatus struct {
	BackendID        string        `json:"backend_id"`
	URL              string        `json:"url"`
	Healthy          bool          `json:"healthy"`
	LastCheck        time.Time     `json:"last_check"`
	LastSuccess      time.Time     `json:"last_success"`
	LastError        string        `json:"last_error,omitempty"`
	ResponseTime     time.Duration `json:"response_time"`
	DNSResolved      bool          `json:"dns_resolved"`
	ResolvedIPs      []string      `json:"resolved_ips,omitempty"`
	LastRequest      time.Time     `json:"last_request"`
	ChecksSkipped    int64         `json:"checks_skipped"`
	TotalChecks      int64         `json:"total_checks"`
	SuccessfulChecks int64         `json:"successful_checks"`
	// Circuit breaker status
	CircuitBreakerOpen  bool   `json:"circuit_breaker_open"`
	CircuitBreakerState string `json:"circuit_breaker_state,omitempty"`
	CircuitFailureCount int    `json:"circuit_failure_count,omitempty"`
	// Health check result (independent of circuit breaker status)
	HealthCheckPassing bool `json:"health_check_passing"`
}

// HealthCircuitBreakerInfo provides circuit breaker status information for health checks.
type HealthCircuitBreakerInfo struct {
	IsOpen       bool
	State        string
	FailureCount int
}

// CircuitBreakerProvider defines a function to get circuit breaker information for a backend.
type CircuitBreakerProvider func(backendID string) *HealthCircuitBreakerInfo

// HealthEventEmitter is a callback used to emit backend health events.
// Accepts event type and a data map for the event payload.
type HealthEventEmitter func(eventType string, data map[string]interface{})

// HealthChecker manages health checking for backend services.
type HealthChecker struct {
	config                 *HealthCheckConfig
	httpClient             *http.Client
	logger                 *slog.Logger
	backends               map[string]string // backend_id -> base_url (internal copy, immutable unless via UpdateBackends)
	healthStatus           map[string]*HealthStatus
	statusMutex            sync.RWMutex
	requestTimes           map[string]time.Time // backend_id -> last_request_time
	requestMutex           sync.RWMutex
	stopChan               chan struct{}
	wg                     sync.WaitGroup
	running                bool
	runningMutex           sync.RWMutex
	circuitBreakerProvider CircuitBreakerProvider
	eventEmitter           HealthEventEmitter // optional emitter for backend health events

	// Internal immutable copies (protected by configMutex during replacement) to avoid races when external config maps mutate
	configMutex              sync.RWMutex
	healthEndpoints          map[string]string
	backendHealthCheckConfig map[string]BackendHealthConfig
	expectedStatusCodes      []int
}

// NewHealthChecker creates a new health checker with the given configuration.
func NewHealthChecker(config *HealthCheckConfig, backends map[string]string, httpClient *http.Client, logger *slog.Logger) *HealthChecker {
	// Create defensive copies of mutable maps to avoid external concurrent mutation races.
	backendsCopy := make(map[string]string, len(backends))
	for k, v := range backends {
		backendsCopy[k] = v
	}
	healthEndpointsCopy := make(map[string]string, len(config.HealthEndpoints))
	for k, v := range config.HealthEndpoints {
		healthEndpointsCopy[k] = v
	}
	backendHealthCfgCopy := make(map[string]BackendHealthConfig, len(config.BackendHealthCheckConfig))
	for k, v := range config.BackendHealthCheckConfig {
		backendHealthCfgCopy[k] = v
	}
	expectedCodesCopy := make([]int, len(config.ExpectedStatusCodes))
	copy(expectedCodesCopy, config.ExpectedStatusCodes)

	return &HealthChecker{
		config:                   config,
		httpClient:               httpClient,
		logger:                   logger,
		backends:                 backendsCopy,
		healthStatus:             make(map[string]*HealthStatus),
		requestTimes:             make(map[string]time.Time),
		stopChan:                 make(chan struct{}),
		healthEndpoints:          healthEndpointsCopy,
		backendHealthCheckConfig: backendHealthCfgCopy,
		expectedStatusCodes:      expectedCodesCopy,
	}
}

// UpdateHealthConfig replaces internal copies of health-related configuration maps atomically.
func (hc *HealthChecker) UpdateHealthConfig(ctx context.Context, cfg *HealthCheckConfig) {
	if cfg == nil {
		return
	}
	healthEndpointsCopy := make(map[string]string, len(cfg.HealthEndpoints))
	for k, v := range cfg.HealthEndpoints {
		healthEndpointsCopy[k] = v
	}
	backendHealthCfgCopy := make(map[string]BackendHealthConfig, len(cfg.BackendHealthCheckConfig))
	for k, v := range cfg.BackendHealthCheckConfig {
		backendHealthCfgCopy[k] = v
	}
	expectedCodesCopy := make([]int, len(cfg.ExpectedStatusCodes))
	copy(expectedCodesCopy, cfg.ExpectedStatusCodes)
	hc.configMutex.Lock()
	hc.healthEndpoints = healthEndpointsCopy
	hc.backendHealthCheckConfig = backendHealthCfgCopy
	hc.expectedStatusCodes = expectedCodesCopy
	hc.configMutex.Unlock()
	hc.logger.DebugContext(ctx, "Health checker config updated", "health_endpoints", len(healthEndpointsCopy), "backend_specific", len(backendHealthCfgCopy))
}

// SetCircuitBreakerProvider sets the circuit breaker provider function.
func (hc *HealthChecker) SetCircuitBreakerProvider(provider CircuitBreakerProvider) {
	hc.circuitBreakerProvider = provider
}

// SetEventEmitter sets the callback used to emit health events.
func (hc *HealthChecker) SetEventEmitter(emitter HealthEventEmitter) {
	hc.eventEmitter = emitter
}

// Start begins the health checking process.
func (hc *HealthChecker) Start(ctx context.Context) error {
	hc.runningMutex.Lock()
	if hc.running {
		hc.runningMutex.Unlock()
		return nil // Already running
	}
	hc.running = true

	// Create a new stop channel if the old one was closed
	select {
	case <-hc.stopChan:
		// Channel is closed, create a new one
		hc.stopChan = make(chan struct{})
	default:
		// Channel is still open, use it
	}

	hc.runningMutex.Unlock()

	// Perform initial health check for all backends
	for backendID, baseURL := range hc.backends {
		hc.initializeBackendStatus(backendID, baseURL)
		// Perform immediate health check
		hc.performHealthCheck(ctx, backendID, baseURL)
	}

	// Start periodic health checks
	for backendID, baseURL := range hc.backends {
		hc.wg.Add(1)
		go hc.runPeriodicHealthCheck(ctx, backendID, baseURL)
	}

	hc.logger.InfoContext(ctx, "Health checker started", "backends", len(hc.backends))
	return nil
}

// Stop stops the health checking process.
func (hc *HealthChecker) Stop(ctx context.Context) {
	hc.runningMutex.Lock()
	if !hc.running {
		hc.runningMutex.Unlock()
		return
	}
	hc.running = false
	hc.runningMutex.Unlock()

	// Close the stop channel only once
	select {
	case <-hc.stopChan:
		// Channel already closed
	default:
		close(hc.stopChan)
	}

	hc.wg.Wait()
	hc.logger.InfoContext(ctx, "Health checker stopped")
}

// IsRunning returns whether the health checker is currently running.
func (hc *HealthChecker) IsRunning() bool {
	hc.runningMutex.RLock()
	defer hc.runningMutex.RUnlock()
	return hc.running
}

// GetHealthStatus returns the current health status for all backends.
func (hc *HealthChecker) GetHealthStatus() map[string]*HealthStatus {
	hc.statusMutex.Lock()
	defer hc.statusMutex.Unlock()

	// Update circuit breaker information for all backends before returning status
	if hc.circuitBreakerProvider != nil {
		for backendID, status := range hc.healthStatus {
			if cbInfo := hc.circuitBreakerProvider(backendID); cbInfo != nil {
				status.CircuitBreakerOpen = cbInfo.IsOpen
				status.CircuitBreakerState = cbInfo.State
				status.CircuitFailureCount = cbInfo.FailureCount
				// Update overall health status considering circuit breaker
				status.Healthy = status.HealthCheckPassing && !status.CircuitBreakerOpen
			}
		}
	}

	result := make(map[string]*HealthStatus)
	for id, status := range hc.healthStatus {
		// Create a copy to avoid race conditions
		statusCopy := *status
		result[id] = &statusCopy
	}
	return result
}

// GetBackendHealthStatus returns the health status for a specific backend.
func (hc *HealthChecker) GetBackendHealthStatus(backendID string) (*HealthStatus, bool) {
	hc.statusMutex.Lock()
	defer hc.statusMutex.Unlock()

	status, exists := hc.healthStatus[backendID]
	if !exists {
		return nil, false
	}

	// Update circuit breaker information for this backend before returning status
	if hc.circuitBreakerProvider != nil {
		if cbInfo := hc.circuitBreakerProvider(backendID); cbInfo != nil {
			status.CircuitBreakerOpen = cbInfo.IsOpen
			status.CircuitBreakerState = cbInfo.State
			status.CircuitFailureCount = cbInfo.FailureCount
			// Update overall health status considering circuit breaker
			status.Healthy = status.HealthCheckPassing && !status.CircuitBreakerOpen
		}
	}

	// Return a copy to avoid race conditions
	statusCopy := *status
	return &statusCopy, true
}

// RecordBackendRequest records that a request was made to a backend.
func (hc *HealthChecker) RecordBackendRequest(backendID string) {
	hc.requestMutex.Lock()
	hc.requestTimes[backendID] = time.Now()
	hc.requestMutex.Unlock()

	// Update last request time in health status
	hc.statusMutex.Lock()
	if status, exists := hc.healthStatus[backendID]; exists {
		status.LastRequest = time.Now()
	}
	hc.statusMutex.Unlock()
}

// initializeBackendStatus initializes the health status for a backend.
func (hc *HealthChecker) initializeBackendStatus(backendID, baseURL string) {
	hc.statusMutex.Lock()
	defer hc.statusMutex.Unlock()

	hc.healthStatus[backendID] = &HealthStatus{
		BackendID:   backendID,
		URL:         baseURL,
		Healthy:     false, // Start as unhealthy until first check
		LastCheck:   time.Time{},
		LastSuccess: time.Time{},
		LastError:   "",
		DNSResolved: false,
		ResolvedIPs: []string{},
		LastRequest: time.Time{},
	}
}

// runPeriodicHealthCheck runs periodic health checks for a backend.
func (hc *HealthChecker) runPeriodicHealthCheck(ctx context.Context, backendID, baseURL string) {
	defer hc.wg.Done()

	interval := hc.getBackendInterval(backendID)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hc.stopChan:
			return
		case <-ticker.C:
			hc.performHealthCheck(ctx, backendID, baseURL)
		}
	}
}

// performHealthCheck performs a health check for a specific backend.
func (hc *HealthChecker) performHealthCheck(ctx context.Context, backendID, baseURL string) {
	start := time.Now()

	// Check if we should skip this check due to recent request
	if hc.shouldSkipHealthCheck(backendID) {
		hc.statusMutex.Lock()
		if status, exists := hc.healthStatus[backendID]; exists {
			status.ChecksSkipped++
		}
		hc.statusMutex.Unlock()
		return
	}

	// Check if backend-specific health checking is disabled
	if !hc.isBackendHealthCheckEnabled(backendID) {
		return
	}

	hc.statusMutex.Lock()
	if status, exists := hc.healthStatus[backendID]; exists {
		status.TotalChecks++
	}
	hc.statusMutex.Unlock()

	// Perform DNS resolution check
	dnsResolved, resolvedIPs, dnsErr := hc.performDNSCheck(ctx, baseURL)

	// Perform HTTP health check
	healthy, responseTime, httpErr := hc.performHTTPCheck(ctx, backendID, baseURL)

	// Update health status
	hc.updateHealthStatus(backendID, healthy, responseTime, dnsResolved, resolvedIPs, dnsErr, httpErr)

	duration := time.Since(start)
	hc.logger.DebugContext(ctx, "Health check completed",
		"backend", backendID,
		"healthy", healthy,
		"dns_resolved", dnsResolved,
		"response_time", responseTime,
		"total_duration", duration)
}

// shouldSkipHealthCheck determines if a health check should be skipped due to recent request.
func (hc *HealthChecker) shouldSkipHealthCheck(backendID string) bool {
	hc.requestMutex.RLock()
	lastRequest, exists := hc.requestTimes[backendID]
	hc.requestMutex.RUnlock()

	if !exists {
		return false
	}

	threshold := hc.config.RecentRequestThreshold
	if threshold <= 0 {
		return false
	}

	return time.Since(lastRequest) < threshold
}

// performDNSCheck performs DNS resolution check for a backend URL.
func (hc *HealthChecker) performDNSCheck(ctx context.Context, baseURL string) (bool, []string, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return false, nil, fmt.Errorf("invalid URL: %w", err)
	}

	host := parsedURL.Hostname()
	if host == "" {
		return false, nil, ErrNoHostname
	}

	// Perform DNS lookup using context-aware resolver
	resolver := &net.Resolver{}
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return false, nil, fmt.Errorf("DNS lookup failed: %w", err)
	}

	resolvedIPs := make([]string, len(ips))
	for i, ip := range ips {
		resolvedIPs[i] = ip.IP.String()
	}

	return true, resolvedIPs, nil
}

// performHTTPCheck performs HTTP health check for a backend.
func (hc *HealthChecker) performHTTPCheck(ctx context.Context, backendID, baseURL string) (bool, time.Duration, error) {
	// Get the health check endpoint
	healthEndpoint := hc.getHealthCheckEndpoint(backendID, baseURL)

	// Create request context with timeout
	timeout := hc.getBackendTimeout(backendID)
	healthCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create HTTP request
	req, err := http.NewRequestWithContext(healthCtx, "GET", healthEndpoint, nil)
	if err != nil {
		return false, 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Add health check headers
	req.Header.Set("User-Agent", "modular-reverseproxy-health-check/1.0")
	req.Header.Set("Accept", "*/*")

	// Perform the request
	start := time.Now()
	resp, err := hc.httpClient.Do(req)
	responseTime := time.Since(start)

	if err != nil {
		return false, responseTime, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check if status code is expected
	expectedCodes := hc.getExpectedStatusCodes(backendID)
	healthy := false
	for _, code := range expectedCodes {
		if resp.StatusCode == code {
			healthy = true
			break
		}
	}

	if !healthy {
		return false, responseTime, fmt.Errorf("%w: %d", ErrUnexpectedStatusCode, resp.StatusCode)
	}

	return true, responseTime, nil
}

// updateHealthStatus updates the health status for a backend.
func (hc *HealthChecker) updateHealthStatus(backendID string, healthy bool, responseTime time.Duration, dnsResolved bool, resolvedIPs []string, dnsErr, httpErr error) {
	hc.statusMutex.Lock()
	defer hc.statusMutex.Unlock()

	// INSERTED: capture previous healthy state
	status, exists := hc.healthStatus[backendID]
	if !exists {
		return
	}
	prevHealthy := status.Healthy

	status.LastCheck = time.Now()
	status.ResponseTime = responseTime
	status.DNSResolved = dnsResolved
	status.ResolvedIPs = resolvedIPs

	// Store health check result (independent of circuit breaker)
	healthCheckPassing := healthy && dnsResolved
	status.HealthCheckPassing = healthCheckPassing

	// Get circuit breaker information if provider is available
	if hc.circuitBreakerProvider != nil {
		if cbInfo := hc.circuitBreakerProvider(backendID); cbInfo != nil {
			status.CircuitBreakerOpen = cbInfo.IsOpen
			status.CircuitBreakerState = cbInfo.State
			status.CircuitFailureCount = cbInfo.FailureCount
		}
	}

	// A backend is overall healthy if health check passes AND circuit breaker is not open
	status.Healthy = healthCheckPassing && !status.CircuitBreakerOpen

	if healthCheckPassing {
		status.LastSuccess = time.Now()
		status.LastError = ""
		status.SuccessfulChecks++
	} else {
		// Record the error
		if dnsErr != nil {
			status.LastError = dnsErr.Error()
		} else if httpErr != nil {
			status.LastError = httpErr.Error()
		}
	}

	// After computing status.Healthy, emit events on transitions
	if hc.eventEmitter != nil && prevHealthy != status.Healthy {
		if status.Healthy {
			hc.eventEmitter(EventTypeBackendHealthy, map[string]interface{}{"backend": backendID})
		} else {
			hc.eventEmitter(EventTypeBackendUnhealthy, map[string]interface{}{"backend": backendID, "error": status.LastError})
		}
	}
}

// getHealthCheckEndpoint returns the health check endpoint for a backend.

func (hc *HealthChecker) getHealthCheckEndpoint(backendID, baseURL string) string {
	hc.configMutex.RLock()
	backendHealthCfg := hc.backendHealthCheckConfig
	healthEndpoints := hc.healthEndpoints
	hc.configMutex.RUnlock()

	// Check for backend-specific health endpoint
	if backendConfig, exists := backendHealthCfg[backendID]; exists && backendConfig.Endpoint != "" {
		// If it's a full URL, use it as is
		if parsedURL, err := url.Parse(backendConfig.Endpoint); err == nil && parsedURL.Scheme != "" {
			return backendConfig.Endpoint
		}
		// Otherwise, treat it as a path and append to base URL
		baseURL, err := url.Parse(baseURL)
		if err != nil {
			return backendConfig.Endpoint // fallback to the endpoint as-is
		}
		baseURL.Path = path.Join(baseURL.Path, backendConfig.Endpoint)
		return baseURL.String()
	}

	// Check for global health endpoint override
	if globalEndpoint, exists := healthEndpoints[backendID]; exists {
		// If it's a full URL, use it as is
		if parsedURL, err := url.Parse(globalEndpoint); err == nil && parsedURL.Scheme != "" {
			return globalEndpoint
		}
		// Otherwise, treat it as a path and append to base URL
		baseURL, err := url.Parse(baseURL)
		if err != nil {
			return globalEndpoint // fallback to the endpoint as-is
		}
		baseURL.Path = path.Join(baseURL.Path, globalEndpoint)
		return baseURL.String()
	}

	// Default to base URL
	return baseURL
}

// getBackendInterval returns the health check interval for a backend.
func (hc *HealthChecker) getBackendInterval(backendID string) time.Duration {
	hc.configMutex.RLock()
	backendHealthCfg := hc.backendHealthCheckConfig
	interval := hc.config.Interval
	hc.configMutex.RUnlock()
	if backendConfig, exists := backendHealthCfg[backendID]; exists && backendConfig.Interval > 0 {
		return backendConfig.Interval
	}
	return interval
}

// getBackendTimeout returns the health check timeout for a backend.
func (hc *HealthChecker) getBackendTimeout(backendID string) time.Duration {
	hc.configMutex.RLock()
	backendHealthCfg := hc.backendHealthCheckConfig
	timeout := hc.config.Timeout
	hc.configMutex.RUnlock()
	if backendConfig, exists := backendHealthCfg[backendID]; exists && backendConfig.Timeout > 0 {
		return backendConfig.Timeout
	}
	return timeout
}

// getExpectedStatusCodes returns the expected status codes for a backend.
func (hc *HealthChecker) getExpectedStatusCodes(backendID string) []int {
	hc.configMutex.RLock()
	backendHealthCfg := hc.backendHealthCheckConfig
	expected := hc.expectedStatusCodes
	hc.configMutex.RUnlock()
	if backendConfig, exists := backendHealthCfg[backendID]; exists && len(backendConfig.ExpectedStatusCodes) > 0 {
		return backendConfig.ExpectedStatusCodes
	}
	if len(expected) > 0 {
		return expected
	}
	return []int{200}
}

// isBackendHealthCheckEnabled returns whether health checking is enabled for a backend.
func (hc *HealthChecker) isBackendHealthCheckEnabled(backendID string) bool {
	hc.configMutex.RLock()
	backendHealthCfg := hc.backendHealthCheckConfig
	hc.configMutex.RUnlock()
	if backendConfig, exists := backendHealthCfg[backendID]; exists {
		return backendConfig.Enabled
	}
	return true
}

// UpdateBackends updates the list of backends to monitor.
func (hc *HealthChecker) UpdateBackends(ctx context.Context, backends map[string]string) {
	// Clone incoming map first
	cloned := make(map[string]string, len(backends))
	for k, v := range backends {
		cloned[k] = v
	}

	hc.statusMutex.Lock()
	// Remove health status for backends that no longer exist
	for backendID := range hc.healthStatus {
		if _, exists := cloned[backendID]; !exists {
			delete(hc.healthStatus, backendID)
			hc.logger.DebugContext(ctx, "Removed health status for backend", "backend", backendID)
		}
	}
	// Track new backends to start goroutines after releasing status lock if running
	newBackends := make(map[string]string)
	for backendID, baseURL := range cloned {
		if _, exists := hc.healthStatus[backendID]; !exists {
			hc.healthStatus[backendID] = &HealthStatus{
				BackendID:   backendID,
				URL:         baseURL,
				Healthy:     false,
				LastCheck:   time.Time{},
				LastSuccess: time.Time{},
				LastError:   "",
				DNSResolved: false,
				ResolvedIPs: []string{},
				LastRequest: time.Time{},
			}
			newBackends[backendID] = baseURL
			hc.logger.DebugContext(ctx, "Added health status for new backend", "backend", backendID)
		}
	}
	hc.backends = cloned
	hc.statusMutex.Unlock()

	// If running, start periodic checks for new backends
	if len(newBackends) > 0 && hc.IsRunning() {
		for backendID, baseURL := range newBackends {
			hc.wg.Add(1)
			go hc.runPeriodicHealthCheck(ctx, backendID, baseURL)
		}
	}
}

// OverallHealthStatus represents the overall health status of the service.
type OverallHealthStatus struct {
	Healthy           bool                     `json:"healthy"`
	TotalBackends     int                      `json:"total_backends"`
	HealthyBackends   int                      `json:"healthy_backends"`
	UnhealthyBackends int                      `json:"unhealthy_backends"`
	CircuitOpenCount  int                      `json:"circuit_open_count"`
	LastCheck         time.Time                `json:"last_check"`
	BackendDetails    map[string]*HealthStatus `json:"backend_details,omitempty"`
}

// GetOverallHealthStatus returns the overall health status of all backends.
// The service is considered healthy if all configured backends are healthy.
func (hc *HealthChecker) GetOverallHealthStatus(includeDetails bool) *OverallHealthStatus {
	allStatus := hc.GetHealthStatus()

	overall := &OverallHealthStatus{
		TotalBackends:  len(allStatus),
		LastCheck:      time.Now(),
		BackendDetails: make(map[string]*HealthStatus),
	}

	healthyCount := 0
	circuitOpenCount := 0

	for backendID, status := range allStatus {
		if status.Healthy {
			healthyCount++
		}
		if status.CircuitBreakerOpen {
			circuitOpenCount++
		}

		if includeDetails {
			overall.BackendDetails[backendID] = status
		}
	}

	overall.HealthyBackends = healthyCount
	overall.UnhealthyBackends = overall.TotalBackends - healthyCount
	overall.CircuitOpenCount = circuitOpenCount
	overall.Healthy = healthyCount == overall.TotalBackends && overall.TotalBackends > 0

	return overall
}
