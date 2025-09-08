package modular

import (
	"context"
	"fmt"
	"time"
)

// Health interface standardization utilities
//
// This file provides adapters and utilities to help migrate from the legacy
// HealthReporter interface to the standardized HealthProvider interface.

// NewHealthReporterAdapter creates a HealthProvider adapter for legacy HealthReporter implementations.
// This allows existing HealthReporter implementations to work with the new standardized interface.
//
// Parameters:
//   - reporter: The legacy HealthReporter implementation
//   - moduleName: The module name to use in the generated HealthReport
//
// The adapter will:
//   - Convert HealthResult to HealthReport format
//   - Use HealthCheckName() as the component name
//   - Respect HealthCheckTimeout() for context timeout
//   - Handle context cancellation appropriately
func NewHealthReporterAdapter(reporter HealthReporter, moduleName string) HealthProvider {
	return &healthReporterAdapter{
		reporter:   reporter,
		moduleName: moduleName,
	}
}

type healthReporterAdapter struct {
	reporter   HealthReporter
	moduleName string
}

func (a *healthReporterAdapter) HealthCheck(ctx context.Context) ([]HealthReport, error) {
	// Create a timeout context based on the reporter's timeout
	timeout := a.reporter.HealthCheckTimeout()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Call the legacy CheckHealth method
	result := a.reporter.CheckHealth(ctx)

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, fmt.Errorf("context cancelled during health check: %w", ctx.Err())
	}

	// Convert HealthResult to HealthReport
	report := HealthReport{
		Module:        a.moduleName,
		Component:     a.reporter.HealthCheckName(),
		Status:        result.Status,
		Message:       result.Message,
		CheckedAt:     result.Timestamp,
		ObservedSince: result.Timestamp, // Use same timestamp for initial observation
		Optional:      false,            // Legacy reporters are assumed to be required
		// Note: Legacy HealthResult doesn't have Details, so we leave it empty
	}

	return []HealthReport{report}, nil
}

// NewSimpleHealthProvider creates a HealthProvider for simple health checks.
// This is useful for creating lightweight health providers without implementing
// the full interface.
//
// Parameters:
//   - moduleName: The module name for the health report
//   - componentName: The component name for the health report
//   - checkFunc: A function that performs the actual health check
//
// The checkFunc receives a context and should return:
//   - HealthStatus: The health status
//   - string: A message describing the health status
//   - error: Any error that occurred during the check
func NewSimpleHealthProvider(moduleName, componentName string, checkFunc func(context.Context) (HealthStatus, string, error)) HealthProvider {
	return &simpleHealthProvider{
		moduleName:    moduleName,
		componentName: componentName,
		checkFunc:     checkFunc,
	}
}

type simpleHealthProvider struct {
	moduleName    string
	componentName string
	checkFunc     func(context.Context) (HealthStatus, string, error)
}

func (p *simpleHealthProvider) HealthCheck(ctx context.Context) ([]HealthReport, error) {
	status, message, err := p.checkFunc(ctx)
	if err != nil {
		// If the check function returns an error, we still create a report
		// but mark it as unhealthy with the error message
		// Intentionally return nil error here since we want to return a health report
		// rather than propagate the check error - this is the expected behavior
		report := HealthReport{
			Module:        p.moduleName,
			Component:     p.componentName,
			Status:        HealthStatusUnhealthy,
			Message:       err.Error(),
			CheckedAt:     time.Now(),
			ObservedSince: time.Now(),
			Optional:      false,
		}
		return []HealthReport{report}, nil //nolint:nilerr // intentional: health check errors become unhealthy status
	}

	report := HealthReport{
		Module:        p.moduleName,
		Component:     p.componentName,
		Status:        status,
		Message:       message,
		CheckedAt:     time.Now(),
		ObservedSince: time.Now(),
		Optional:      false,
	}

	return []HealthReport{report}, nil
}

// NewStaticHealthProvider creates a HealthProvider that always returns the same status.
// This is useful for testing or for components that have a fixed health status.
//
// Parameters:
//   - moduleName: The module name for the health report
//   - componentName: The component name for the health report
//   - status: The health status to always return
//   - message: The message to always return
func NewStaticHealthProvider(moduleName, componentName string, status HealthStatus, message string) HealthProvider {
	return &staticHealthProvider{
		moduleName:    moduleName,
		componentName: componentName,
		status:        status,
		message:       message,
	}
}

type staticHealthProvider struct {
	moduleName    string
	componentName string
	status        HealthStatus
	message       string
}

func (p *staticHealthProvider) HealthCheck(ctx context.Context) ([]HealthReport, error) {
	report := HealthReport{
		Module:        p.moduleName,
		Component:     p.componentName,
		Status:        p.status,
		Message:       p.message,
		CheckedAt:     time.Now(),
		ObservedSince: time.Now(),
		Optional:      false,
	}

	return []HealthReport{report}, nil
}

// NewCompositeHealthProvider creates a HealthProvider that combines multiple providers.
// This allows you to aggregate health reports from multiple sources into a single provider.
//
// All provided HealthProviders will be called and their reports combined.
// If any provider returns an error, the composite will return that error.
func NewCompositeHealthProvider(providers ...HealthProvider) HealthProvider {
	return &compositeHealthProvider{
		providers: providers,
	}
}

type compositeHealthProvider struct {
	providers []HealthProvider
}

func (p *compositeHealthProvider) HealthCheck(ctx context.Context) ([]HealthReport, error) {
	var allReports []HealthReport

	for _, provider := range p.providers {
		reports, err := provider.HealthCheck(ctx)
		if err != nil {
			return nil, fmt.Errorf("health check failed for provider: %w", err)
		}
		allReports = append(allReports, reports...)
	}

	return allReports, nil
}

// Migration utilities

// HealthReporterToProvider converts a HealthReporter to a HealthProvider using the adapter.
// This is a convenience function for the common case of adapting a single reporter.
//
// Deprecated: Use NewHealthReporterAdapter directly for better clarity.
func HealthReporterToProvider(reporter HealthReporter, moduleName string) HealthProvider {
	return NewHealthReporterAdapter(reporter, moduleName)
}

// MustImplementHealthProvider is a compile-time check to ensure a type implements HealthProvider.
// This can be used in tests or during development to verify interface compliance.
//
// Usage:
//
//	var _ HealthProvider = (*YourType)(nil) // Add this line to verify YourType implements HealthProvider
var MustImplementHealthProvider = func(HealthProvider) {}

// MustImplementHealthReporter is a compile-time check for HealthReporter (legacy interface).
// This helps during migration to identify which types implement the legacy interface.
//
// Usage:
//
//	var _ HealthReporter = (*YourLegacyType)(nil) // Add this line to verify YourLegacyType implements HealthReporter
var MustImplementHealthReporter = func(HealthReporter) {}
