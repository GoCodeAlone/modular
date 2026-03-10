package modular

import (
	"context"
	"fmt"
	"time"
)

// HealthStatus represents the health state of a component.
type HealthStatus int

const (
	// StatusUnknown indicates the health state has not been determined.
	StatusUnknown HealthStatus = iota
	// StatusHealthy indicates the component is functioning normally.
	StatusHealthy
	// StatusDegraded indicates the component is functioning with reduced capability.
	StatusDegraded
	// StatusUnhealthy indicates the component is not functioning.
	StatusUnhealthy
)

// String returns the string representation of a HealthStatus.
func (s HealthStatus) String() string {
	switch s {
	case StatusUnknown:
		return "unknown"
	case StatusHealthy:
		return "healthy"
	case StatusDegraded:
		return "degraded"
	case StatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// IsHealthy returns true if the status is StatusHealthy.
func (s HealthStatus) IsHealthy() bool {
	return s == StatusHealthy
}

// HealthProvider is an interface for components that can report their health.
type HealthProvider interface {
	HealthCheck(ctx context.Context) ([]HealthReport, error)
}

// HealthReport represents the health status of a single component.
type HealthReport struct {
	Module        string
	Component     string
	Status        HealthStatus
	Message       string
	CheckedAt     time.Time
	ObservedSince time.Time
	Optional      bool
	Details       map[string]any
}

// AggregatedHealth represents the combined health of all providers.
type AggregatedHealth struct {
	Readiness   HealthStatus
	Health      HealthStatus
	Reports     []HealthReport
	GeneratedAt time.Time
}

// forceHealthRefreshKeyType is an unexported type for context key safety.
type forceHealthRefreshKeyType struct{}

// ForceHealthRefreshKey is the context key used to force a health refresh,
// bypassing the cache. Usage: context.WithValue(ctx, modular.ForceHealthRefreshKey, true)
var ForceHealthRefreshKey = forceHealthRefreshKeyType{}

// simpleHealthProvider adapts a function into a HealthProvider.
type simpleHealthProvider struct {
	module    string
	component string
	fn        func(ctx context.Context) (HealthStatus, string, error)
}

// NewSimpleHealthProvider creates a HealthProvider from a function that returns
// a status, message, and error.
func NewSimpleHealthProvider(module, component string, fn func(ctx context.Context) (HealthStatus, string, error)) HealthProvider {
	return &simpleHealthProvider{module: module, component: component, fn: fn}
}

func (p *simpleHealthProvider) HealthCheck(ctx context.Context) ([]HealthReport, error) {
	status, msg, err := p.fn(ctx)
	report := HealthReport{
		Module:    p.module,
		Component: p.component,
		Status:    status,
		Message:   msg,
		CheckedAt: time.Now(),
	}
	return []HealthReport{report}, err
}

// staticHealthProvider returns fixed reports.
type staticHealthProvider struct {
	reports []HealthReport
}

// NewStaticHealthProvider creates a HealthProvider that always returns the given reports.
func NewStaticHealthProvider(reports ...HealthReport) HealthProvider {
	return &staticHealthProvider{reports: reports}
}

func (p *staticHealthProvider) HealthCheck(_ context.Context) ([]HealthReport, error) {
	now := time.Now()
	result := make([]HealthReport, len(p.reports))
	copy(result, p.reports)
	for i := range result {
		result[i].CheckedAt = now
	}
	return result, nil
}

// compositeHealthProvider aggregates multiple providers into one.
type compositeHealthProvider struct {
	providers []HealthProvider
}

// NewCompositeHealthProvider creates a HealthProvider that delegates to multiple providers.
func NewCompositeHealthProvider(providers ...HealthProvider) HealthProvider {
	return &compositeHealthProvider{providers: providers}
}

func (p *compositeHealthProvider) HealthCheck(ctx context.Context) ([]HealthReport, error) {
	var all []HealthReport
	for _, provider := range p.providers {
		reports, err := provider.HealthCheck(ctx)
		if err != nil {
			return all, fmt.Errorf("composite health check: %w", err)
		}
		all = append(all, reports...)
	}
	return all, nil
}
