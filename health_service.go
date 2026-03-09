package modular

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AggregateHealthService collects health reports from registered providers
// and produces an aggregated health result with caching and event emission.
type AggregateHealthService struct {
	providers   map[string]HealthProvider
	mu          sync.RWMutex
	cache       *AggregatedHealth
	cacheMu     sync.RWMutex
	cacheExpiry time.Time
	cacheTTL    time.Duration
	lastStatus  HealthStatus
	subject     Subject
	logger      Logger
}

// HealthServiceOption configures an AggregateHealthService.
type HealthServiceOption func(*AggregateHealthService)

// WithCacheTTL sets the cache time-to-live for health check results.
func WithCacheTTL(d time.Duration) HealthServiceOption {
	return func(s *AggregateHealthService) {
		s.cacheTTL = d
	}
}

// WithSubject sets the event subject for health event emission.
func WithSubject(sub Subject) HealthServiceOption {
	return func(s *AggregateHealthService) {
		s.subject = sub
	}
}

// WithHealthLogger sets the structured logger for the health service.
func WithHealthLogger(l Logger) HealthServiceOption {
	return func(s *AggregateHealthService) {
		s.logger = l
	}
}

// NewAggregateHealthService creates a new AggregateHealthService with the given options.
func NewAggregateHealthService(opts ...HealthServiceOption) *AggregateHealthService {
	svc := &AggregateHealthService{
		providers:  make(map[string]HealthProvider),
		cacheTTL:   250 * time.Millisecond,
		lastStatus: StatusUnknown,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// AddProvider registers a named health provider and invalidates the cache.
func (s *AggregateHealthService) AddProvider(name string, provider HealthProvider) {
	s.mu.Lock()
	s.providers[name] = provider
	s.mu.Unlock()
	s.invalidateCache()
}

// RemoveProvider removes a named health provider and invalidates the cache.
func (s *AggregateHealthService) RemoveProvider(name string) {
	s.mu.Lock()
	delete(s.providers, name)
	s.mu.Unlock()
	s.invalidateCache()
}

func (s *AggregateHealthService) invalidateCache() {
	s.cacheMu.Lock()
	s.cache = nil
	s.cacheExpiry = time.Time{}
	s.cacheMu.Unlock()
}

// providerResult is used to collect results from concurrent provider checks.
type providerResult struct {
	reports []HealthReport
	err     error
	name    string
}

// Check evaluates all registered providers and returns an aggregated health result.
// Results are cached for the configured TTL unless ForceHealthRefreshKey is set in the context.
// The returned AggregatedHealth is a deep copy and safe to mutate.
func (s *AggregateHealthService) Check(ctx context.Context) (*AggregatedHealth, error) {
	// Check cache validity
	forceRefresh, _ := ctx.Value(ForceHealthRefreshKey).(bool)
	if !forceRefresh {
		s.cacheMu.RLock()
		if s.cache != nil && time.Now().Before(s.cacheExpiry) {
			copied := s.deepCopyAggregated(s.cache)
			s.cacheMu.RUnlock()
			return copied, nil
		}
		s.cacheMu.RUnlock()
	}

	// Snapshot providers under read lock
	s.mu.RLock()
	providers := make(map[string]HealthProvider, len(s.providers))
	for k, v := range s.providers {
		providers[k] = v
	}
	s.mu.RUnlock()

	// Fan-out to all providers
	ch := make(chan providerResult, len(providers))
	for name, provider := range providers {
		go func(name string, provider HealthProvider) {
			result := providerResult{name: name}
			defer func() {
				if r := recover(); r != nil {
					result.reports = []HealthReport{{
						Module:    name,
						Component: "panic-recovery",
						Status:    StatusUnhealthy,
						Message:   fmt.Sprintf("provider panicked: %v", r),
						CheckedAt: time.Now(),
					}}
					result.err = nil
					ch <- result
				}
			}()
			reports, err := provider.HealthCheck(ctx)
			result.reports = reports
			result.err = err
			ch <- result
		}(name, provider)
	}

	// Collect results
	var allReports []HealthReport
	readiness := StatusHealthy
	health := StatusHealthy

	for range len(providers) {
		result := <-ch

		if result.err != nil {
			// Check if error is temporary
			status := StatusUnhealthy
			if te, ok := result.err.(interface{ Temporary() bool }); ok && te.Temporary() {
				status = StatusDegraded
			}
			// Add error report
			allReports = append(allReports, HealthReport{
				Module:    result.name,
				Component: "error",
				Status:    status,
				Message:   result.err.Error(),
				CheckedAt: time.Now(),
			})
			readiness = worstStatus(readiness, status)
			health = worstStatus(health, status)
			continue
		}

		for _, report := range result.reports {
			allReports = append(allReports, report)
			health = worstStatus(health, report.Status)
			if !report.Optional {
				readiness = worstStatus(readiness, report.Status)
			}
		}
	}

	aggregated := &AggregatedHealth{
		Readiness:   readiness,
		Health:      health,
		Reports:     allReports,
		GeneratedAt: time.Now(),
	}

	// Cache result
	s.cacheMu.Lock()
	s.cache = aggregated
	s.cacheExpiry = time.Now().Add(s.cacheTTL)
	s.cacheMu.Unlock()

	// Emit events
	s.emitHealthEvaluated(ctx, aggregated)

	s.cacheMu.Lock()
	previousStatus := s.lastStatus
	s.lastStatus = aggregated.Health
	s.cacheMu.Unlock()

	if previousStatus != aggregated.Health {
		s.emitHealthStatusChanged(ctx, previousStatus, aggregated.Health)
	}

	return s.deepCopyAggregated(aggregated), nil
}

// deepCopyAggregated returns a deep copy of an AggregatedHealth, including
// reports and their Details maps, so callers cannot mutate cached state.
func (s *AggregateHealthService) deepCopyAggregated(src *AggregatedHealth) *AggregatedHealth {
	if src == nil {
		return nil
	}
	dst := &AggregatedHealth{
		Readiness:   src.Readiness,
		Health:      src.Health,
		GeneratedAt: src.GeneratedAt,
		Reports:     make([]HealthReport, len(src.Reports)),
	}
	for i, r := range src.Reports {
		dst.Reports[i] = r
		if r.Details != nil {
			dst.Reports[i].Details = make(map[string]any, len(r.Details))
			for k, v := range r.Details {
				dst.Reports[i].Details[k] = v
			}
		}
	}
	return dst
}

func (s *AggregateHealthService) emitHealthEvaluated(ctx context.Context, agg *AggregatedHealth) {
	if s.subject == nil {
		return
	}
	event := NewCloudEvent(EventTypeHealthEvaluated, "modular/health-service", map[string]any{
		"readiness":    agg.Readiness.String(),
		"health":       agg.Health.String(),
		"report_count": len(agg.Reports),
	}, nil)
	if err := s.subject.NotifyObservers(ctx, event); err != nil && s.logger != nil {
		s.logger.Debug("Failed to emit health evaluated event", "error", err)
	}
}

func (s *AggregateHealthService) emitHealthStatusChanged(ctx context.Context, from, to HealthStatus) {
	if s.subject == nil {
		return
	}
	event := NewCloudEvent(EventTypeHealthStatusChanged, "modular/health-service", map[string]any{
		"previous_status": from.String(),
		"current_status":  to.String(),
	}, nil)
	if err := s.subject.NotifyObservers(ctx, event); err != nil && s.logger != nil {
		s.logger.Debug("Failed to emit health status changed event", "error", err)
	}
}

// worstStatus returns the worse of two health statuses.
// StatusUnknown is treated as StatusUnhealthy for aggregation purposes.
// When both normalize to the same severity, StatusUnhealthy is preferred
// over StatusUnknown.
func worstStatus(a, b HealthStatus) HealthStatus {
	ar := normalizeForAggregation(a)
	br := normalizeForAggregation(b)
	if ar > br {
		return a
	}
	if br > ar {
		return b
	}
	// Tie-break: prefer StatusUnhealthy over StatusUnknown
	if a == StatusUnknown && b == StatusUnhealthy {
		return b
	}
	if b == StatusUnknown && a == StatusUnhealthy {
		return a
	}
	return a
}

// normalizeForAggregation maps StatusUnknown to StatusUnhealthy severity for comparison.
func normalizeForAggregation(s HealthStatus) int {
	switch s {
	case StatusHealthy:
		return 0
	case StatusDegraded:
		return 1
	case StatusUnhealthy:
		return 2
	case StatusUnknown:
		return 2 // Unknown treated as Unhealthy
	default:
		return 2
	}
}
