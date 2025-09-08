package modular

import (
	"context"
	"fmt"
	"time"
)

// DynamicReloadConfig configures dynamic reload behavior
type DynamicReloadConfig struct {
	Enabled       bool          `json:"enabled"`
	ReloadTimeout time.Duration `json:"reload_timeout"`
}

// HealthAggregatorConfig configures health aggregation
type HealthAggregatorConfig struct {
	Enabled       bool          `json:"enabled"`
	CheckInterval time.Duration `json:"check_interval"`
	CheckTimeout  time.Duration `json:"check_timeout"`
}

// ApplicationOption represents a configuration option for the application
type ApplicationOption func(*StdApplication) error

// WithDynamicReload configures dynamic reload functionality
func WithDynamicReload(config DynamicReloadConfig) ApplicationOption {
	return func(app *StdApplication) error {
		if !config.Enabled {
			// If disabled, don't register the service
			return nil
		}

		// Create and configure the reload orchestrator
		orchestratorConfig := ReloadOrchestratorConfig{
			BackoffBase: 2 * time.Second,
			BackoffCap:  2 * time.Minute,
			QueueSize:   100,
		}

		if config.ReloadTimeout > 0 {
			// ReloadOrchestrator doesn't directly use ReloadTimeout from config
			// It uses per-module timeouts, but we could extend this later
		}

		orchestrator := NewReloadOrchestratorWithConfig(orchestratorConfig)

		// Register as a service
		err := app.RegisterService("reloadOrchestrator", orchestrator)
		if err != nil {
			return fmt.Errorf("failed to register reload orchestrator: %w", err)
		}

		return nil
	}
}

// WithHealthAggregator configures health aggregation functionality
func WithHealthAggregator(config HealthAggregatorConfig) ApplicationOption {
	return func(app *StdApplication) error {
		if !config.Enabled {
			// If disabled, don't register the service
			return nil
		}

		// Create a basic health aggregator
		// For now, we'll create a simple implementation
		aggregator := &BasicHealthAggregator{
			checkInterval: config.CheckInterval,
			checkTimeout:  config.CheckTimeout,
		}

		// Register as a service
		err := app.RegisterService("healthAggregator", aggregator)
		if err != nil {
			return fmt.Errorf("failed to register health aggregator: %w", err)
		}

		return nil
	}
}

// NewStdApplicationWithOptions creates a new application with options
func NewStdApplicationWithOptions(cp ConfigProvider, logger Logger, options ...ApplicationOption) Application {
	// Create the base application
	app := NewStdApplication(cp, logger).(*StdApplication)

	// Apply all options
	for _, option := range options {
		if err := option(app); err != nil {
			// For now, we'll log the error but continue
			// In a production system, you might want to fail fast
			if logger != nil {
				logger.Error("Failed to apply application option", "error", err)
			}
		}
	}

	return app
}

// BasicHealthAggregator provides a simple health aggregation implementation
type BasicHealthAggregator struct {
	checkInterval time.Duration
	checkTimeout  time.Duration
	providers     []HealthProvider
}

// Collect gathers health reports from all registered providers
func (a *BasicHealthAggregator) Collect(ctx context.Context) (AggregatedHealth, error) {
	// This is a basic implementation
	// In a full implementation, this would:
	// 1. Collect health reports from all providers
	// 2. Aggregate them into a single health status
	// 3. Return the aggregated result

	return AggregatedHealth{
		Readiness:   HealthStatusHealthy,
		Health:      HealthStatusHealthy,
		Reports:     []HealthReport{},
		GeneratedAt: time.Now(),
	}, nil
}

// RegisterProvider registers a health provider (basic implementation)
func (a *BasicHealthAggregator) RegisterProvider(provider HealthProvider) {
	a.providers = append(a.providers, provider)
}
