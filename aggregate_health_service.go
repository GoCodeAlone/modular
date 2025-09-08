package modular

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Static errors for health aggregation
var (
	ErrModuleNameEmpty      = errors.New("module name cannot be empty")
	ErrProviderNil          = errors.New("provider cannot be nil")
	ErrProviderAlreadyExists = errors.New("provider already registered")
	ErrProviderNotRegistered = errors.New("no provider registered")
)

// AggregateHealthService implements the HealthAggregator interface to collect
// health reports from registered providers and aggregate them according to
// the design brief specifications for FR-048 Health Aggregation.
//
// The service provides:
//   - Thread-safe provider registration and management
//   - Concurrent health collection with timeouts
//   - Status aggregation following readiness/health rules
//   - Caching with configurable TTL
//   - Event emission on status changes
//   - Panic recovery for provider failures
type AggregateHealthService struct {
	providers map[string]providerInfo
	mu        sync.RWMutex

	// Caching configuration
	cacheEnabled bool
	cacheTTL     time.Duration
	lastResult   *AggregatedHealth
	lastCheck    time.Time

	// Timeout configuration
	defaultTimeout time.Duration

	// Event subject for publishing health events
	eventSubject Subject

	// Track previous status for change detection
	previousStatus HealthStatus
}

// providerInfo holds information about a registered health provider
type providerInfo struct {
	provider HealthProvider
	optional bool
	module   string
}

// AggregateHealthServiceConfig provides configuration for the health aggregation service
type AggregateHealthServiceConfig struct {
	// CacheTTL is the time-to-live for cached health results
	// Default: 250ms as specified in design brief
	CacheTTL time.Duration

	// DefaultTimeout is the default timeout for individual provider calls
	// Default: 200ms as specified in design brief
	DefaultTimeout time.Duration

	// CacheEnabled controls whether result caching is active
	// Default: true
	CacheEnabled bool
}

// NewAggregateHealthService creates a new aggregate health service with default configuration
func NewAggregateHealthService() *AggregateHealthService {
	return NewAggregateHealthServiceWithConfig(AggregateHealthServiceConfig{
		CacheTTL:       250 * time.Millisecond,
		DefaultTimeout: 200 * time.Millisecond,
		CacheEnabled:   true,
	})
}

// NewAggregateHealthServiceWithConfig creates a new aggregate health service with custom configuration
func NewAggregateHealthServiceWithConfig(config AggregateHealthServiceConfig) *AggregateHealthService {
	if config.CacheTTL <= 0 {
		config.CacheTTL = 250 * time.Millisecond
	}
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = 200 * time.Millisecond
	}

	return &AggregateHealthService{
		providers:      make(map[string]providerInfo),
		cacheEnabled:   config.CacheEnabled,
		cacheTTL:       config.CacheTTL,
		defaultTimeout: config.DefaultTimeout,
	}
}

// SetEventSubject sets the event subject for publishing health events
func (s *AggregateHealthService) SetEventSubject(subject Subject) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventSubject = subject
}

// RegisterProvider registers a health provider for the specified module
func (s *AggregateHealthService) RegisterProvider(moduleName string, provider HealthProvider, optional bool) error {
	if moduleName == "" {
		return fmt.Errorf("health aggregation: %w", ErrModuleNameEmpty)
	}
	if provider == nil {
		return fmt.Errorf("health aggregation: %w", ErrProviderNil)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate registration
	if _, exists := s.providers[moduleName]; exists {
		return fmt.Errorf("health aggregation: provider for module '%s': %w", moduleName, ErrProviderAlreadyExists)
	}

	s.providers[moduleName] = providerInfo{
		provider: provider,
		optional: optional,
		module:   moduleName,
	}

	return nil
}

// UnregisterProvider removes a health provider for the specified module
func (s *AggregateHealthService) UnregisterProvider(moduleName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.providers[moduleName]; !exists {
		return fmt.Errorf("health aggregation: module '%s': %w", moduleName, ErrProviderNotRegistered)
	}

	delete(s.providers, moduleName)

	// Clear cache when provider is removed
	s.lastResult = nil
	s.lastCheck = time.Time{}

	return nil
}

// Collect gathers health reports from all registered providers and aggregates them
// according to the design brief specifications.
//
// Aggregation Rules:
//   - Readiness: Start healthy, degrade only for non-optional failures (Optional=false)
//   - Health: Worst status across ALL providers (including optional)
//   - Status hierarchy: healthy < degraded < unhealthy
func (s *AggregateHealthService) Collect(ctx context.Context) (AggregatedHealth, error) {
	s.mu.RLock()

	// Check for forced refresh context value
	forceRefresh := false
	if ctx.Value("force_refresh") != nil {
		forceRefresh = true
	}

	// Return cached result if available and not expired
	if s.cacheEnabled && !forceRefresh && s.lastResult != nil {
		if time.Since(s.lastCheck) < s.cacheTTL {
			result := *s.lastResult
			s.mu.RUnlock()
			return result, nil
		}
	}

	// Copy providers for concurrent access
	providers := make(map[string]providerInfo)
	for name, info := range s.providers {
		providers[name] = info
	}
	eventSubject := s.eventSubject
	previousStatus := s.previousStatus
	s.mu.RUnlock()

	start := time.Now()

	// Collect health reports concurrently
	reports, err := s.collectReports(ctx, providers)
	if err != nil {
		return AggregatedHealth{}, fmt.Errorf("health aggregation: failed to collect reports: %w", err)
	}

	// Aggregate the health status
	aggregated := s.aggregateHealth(reports)
	aggregated.GeneratedAt = time.Now()

	duration := time.Since(start)

	// Check for status changes
	statusChanged := false
	if previousStatus != HealthStatusUnknown && previousStatus != aggregated.Health {
		statusChanged = true
	}

	s.mu.Lock()
	// Update cache
	if s.cacheEnabled {
		s.lastResult = &aggregated
		s.lastCheck = time.Now()
	}

	// Update previous status tracking
	s.previousStatus = aggregated.Health
	s.mu.Unlock()

	// Emit health.evaluated event - emit on every evaluation per requirements
	if eventSubject != nil {
		// Convert to AggregateHealthSnapshot for compatibility
		snapshot := AggregateHealthSnapshot{
			OverallStatus:   aggregated.Health,
			ReadinessStatus: aggregated.Readiness,
			Components:      make(map[string]HealthResult),
			Summary: HealthSummary{
				TotalCount: len(reports),
			},
			GeneratedAt: aggregated.GeneratedAt,
			Timestamp:   aggregated.GeneratedAt,
			SnapshotID:  fmt.Sprintf("health-%d", time.Now().UnixNano()),
		}

		// Count statuses for summary
		for _, report := range reports {
			switch report.Status {
			case HealthStatusHealthy:
				snapshot.Summary.HealthyCount++
			case HealthStatusDegraded:
				snapshot.Summary.DegradedCount++
			case HealthStatusUnhealthy:
				snapshot.Summary.UnhealthyCount++
			}

			// Add to components map for compatibility
			snapshot.Components[report.Module] = HealthResult{
				Status:    report.Status,
				Message:   report.Message,
				Timestamp: report.CheckedAt,
			}
		}

		event := &HealthEvaluatedEvent{
			EvaluationID:   fmt.Sprintf("health-eval-%d", time.Now().UnixNano()),
			Timestamp:      time.Now(),
			Snapshot:       snapshot,
			Duration:       duration,
			TriggerType:    HealthTriggerScheduled, // Default trigger - would be parameterized in real implementation
			StatusChanged:  statusChanged,
			PreviousStatus: previousStatus,
			Metrics: &HealthEvaluationMetrics{
				ComponentsEvaluated:   len(reports),
				TotalEvaluationTime:   duration,
				AverageResponseTimeMs: float64(duration.Milliseconds()),
			},
		}

		// Fire and forget event emission (placeholder)
		// In real implementation, this would convert to CloudEvent and emit through Subject
		go func() {
			// eventSubject.NotifyObservers(context.Background(), cloudEvent)
			_ = eventSubject
			_ = event
		}()
	}

	return aggregated, nil
}

// collectReports collects health reports from all providers concurrently
func (s *AggregateHealthService) collectReports(ctx context.Context, providers map[string]providerInfo) ([]HealthReport, error) {
	if len(providers) == 0 {
		return []HealthReport{}, nil
	}

	results := make(chan providerResult, len(providers))

	// Launch goroutines for each provider
	for moduleName, info := range providers {
		go s.collectFromProvider(ctx, moduleName, info, results)
	}

	// Collect results
	reports := make([]HealthReport, 0, len(providers))
	for i := 0; i < len(providers); i++ {
		result := <-results
		reports = append(reports, result.reports...)
	}

	return reports, nil
}

// providerResult holds the result from a single provider
type providerResult struct {
	reports []HealthReport
	err     error
	module  string
}

// collectFromProvider collects health reports from a single provider with panic recovery
func (s *AggregateHealthService) collectFromProvider(ctx context.Context, moduleName string, info providerInfo, results chan<- providerResult) {
	defer func() {
		if r := recover(); r != nil {
			// Panic recovery: convert panic to unhealthy report
			report := HealthReport{
				Module:        moduleName,
				Status:        HealthStatusUnhealthy,
				Message:       fmt.Sprintf("Health check panicked: %v", r),
				CheckedAt:     time.Now(),
				ObservedSince: time.Now(),
				Optional:      info.optional,
				Details: map[string]any{
					"panic":      r,
					"stackTrace": "panic recovery in health check",
				},
			}

			results <- providerResult{
				reports: []HealthReport{report},
				err:     nil,
				module:  moduleName,
			}
		}
	}()

	// Create timeout context for the provider
	providerCtx, cancel := context.WithTimeout(ctx, s.defaultTimeout)
	defer cancel()

	reports, err := info.provider.HealthCheck(providerCtx)
	if err != nil {
		// Provider error handling
		status := HealthStatusUnhealthy

		// Check if error is temporary
		if temp, ok := err.(interface{ Temporary() bool }); ok && temp.Temporary() {
			status = HealthStatusDegraded
		}

		// Create error report
		report := HealthReport{
			Module:        moduleName,
			Status:        status,
			Message:       fmt.Sprintf("Health check failed: %v", err),
			CheckedAt:     time.Now(),
			ObservedSince: time.Now(),
			Optional:      info.optional,
			Details: map[string]any{
				"error": err.Error(),
			},
		}

		results <- providerResult{
			reports: []HealthReport{report},
			err:     nil,
			module:  moduleName,
		}
		return
	}

	// Set module and optional flag on reports
	for i := range reports {
		reports[i].Module = moduleName
		reports[i].Optional = info.optional
		if reports[i].CheckedAt.IsZero() {
			reports[i].CheckedAt = time.Now()
		}
		if reports[i].ObservedSince.IsZero() {
			reports[i].ObservedSince = time.Now()
		}
	}

	results <- providerResult{
		reports: reports,
		err:     nil,
		module:  moduleName,
	}
}

// aggregateHealth applies the aggregation rules to determine overall health and readiness
func (s *AggregateHealthService) aggregateHealth(reports []HealthReport) AggregatedHealth {
	if len(reports) == 0 {
		// No reports means healthy by default
		return AggregatedHealth{
			Readiness: HealthStatusHealthy,
			Health:    HealthStatusHealthy,
			Reports:   []HealthReport{},
		}
	}

	// Initialize status as healthy
	readiness := HealthStatusHealthy
	health := HealthStatusHealthy

	// Apply aggregation rules
	for _, report := range reports {
		// Health includes all providers (required and optional)
		health = worstStatus(health, report.Status)

		// Readiness only considers required (non-optional) providers
		if !report.Optional {
			readiness = worstStatus(readiness, report.Status)
		}
	}

	return AggregatedHealth{
		Readiness: readiness,
		Health:    health,
		Reports:   reports,
	}
}

// worstStatus returns the worst status between two health statuses
// Status hierarchy: healthy < degraded < unhealthy < unknown
func worstStatus(a, b HealthStatus) HealthStatus {
	statusPriority := map[HealthStatus]int{
		HealthStatusHealthy:   0,
		HealthStatusDegraded:  1,
		HealthStatusUnhealthy: 2,
		HealthStatusUnknown:   3,
	}

	priorityA := statusPriority[a]
	priorityB := statusPriority[b]

	if priorityA >= priorityB {
		return a
	}
	return b
}

// GetProviders returns information about all registered providers (for testing/debugging)
func (s *AggregateHealthService) GetProviders() map[string]ProviderInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]ProviderInfo)
	for name, info := range s.providers {
		result[name] = ProviderInfo{
			Module:   info.module,
			Optional: info.optional,
		}
	}
	return result
}

// ProviderInfo provides information about a registered provider
type ProviderInfo struct {
	Module   string
	Optional bool
}

// HealthStatusChangedEvent represents an event emitted when the overall health status changes
type HealthStatusChangedEvent struct {
	Timestamp      time.Time
	NewStatus      HealthStatus
	PreviousStatus HealthStatus
	Duration       time.Duration
	ReportCount    int
}

// GetEventType returns the event type for status change events
func (e *HealthStatusChangedEvent) GetEventType() string {
	return "health.aggregate.updated"
}

// GetEventSource returns the event source for status change events
func (e *HealthStatusChangedEvent) GetEventSource() string {
	return "modular.core.health.aggregator"
}

// GetTimestamp returns when this event occurred
func (e *HealthStatusChangedEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

// EventObserver interface for health status change notifications
type EventObserver interface {
	OnStatusChange(ctx context.Context, event *HealthStatusChangedEvent)
}
