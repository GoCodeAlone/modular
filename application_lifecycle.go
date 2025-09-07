// Package modular provides enhanced lifecycle management for the application
package modular

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/GoCodeAlone/modular/config"
	"github.com/GoCodeAlone/modular/health"
	"github.com/GoCodeAlone/modular/lifecycle"
	"github.com/GoCodeAlone/modular/registry"
)

// ApplicationLifecycle provides enhanced lifecycle management for the application
// with integrated configuration validation, service registry population,
// lifecycle event dispatching, health aggregation, and graceful shutdown.
type ApplicationLifecycle struct {
	app                 *StdApplication
	configLoader        config.ConfigLoader
	configValidator     config.ConfigValidator
	serviceRegistry     registry.ServiceRegistry
	lifecycleDispatcher lifecycle.EventDispatcher
	healthAggregator    health.HealthAggregator
	isStarted           bool
	stopTimeout         time.Duration
}

// NewApplicationLifecycle creates a new lifecycle manager for the application
func NewApplicationLifecycle(app *StdApplication) *ApplicationLifecycle {
	al := &ApplicationLifecycle{
		app:         app,
		stopTimeout: 30 * time.Second,
	}

	// Initialize core services
	al.configLoader = config.NewLoader()
	al.configValidator = config.NewValidator()
	al.serviceRegistry = registry.NewRegistry(nil)        // Use default config
	al.lifecycleDispatcher = lifecycle.NewDispatcher(nil) // Use default config
	al.healthAggregator = health.NewAggregator(nil)       // Use default config

	return al
}

// InitializeWithLifecycle performs enhanced initialization with lifecycle events,
// configuration validation gates, and service registry population
func (al *ApplicationLifecycle) InitializeWithLifecycle(ctx context.Context) error {
	// Emit lifecycle event: Initialization started
	if err := al.emitLifecycleEvent(ctx, "initialization.started", nil); err != nil {
		al.app.logger.Error("Failed to emit initialization started event", "error", err)
	}

	// Step 1: Configuration Load + Validation Gate
	if err := al.loadAndValidateConfiguration(ctx); err != nil {
		if emitErr := al.emitLifecycleEvent(ctx, "initialization.failed", map[string]interface{}{
			"error": err.Error(),
			"phase": "configuration",
		}); emitErr != nil {
			al.app.logger.Error("Failed to emit initialization failed event", "error", emitErr)
		}
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Emit lifecycle event: Configuration loaded
	if err := al.emitLifecycleEvent(ctx, "configuration.loaded", nil); err != nil {
		al.app.logger.Error("Failed to emit configuration loaded event", "error", err)
	}

	// Step 2: Resolve dependencies in deterministic order
	moduleOrder, err := al.app.resolveDependencies()
	if err != nil {
		if emitErr := al.emitLifecycleEvent(ctx, "initialization.failed", map[string]interface{}{
			"error": err.Error(),
			"phase": "dependency_resolution",
		}); emitErr != nil {
			al.app.logger.Error("Failed to emit initialization failed event", "error", emitErr)
		}
		return fmt.Errorf("dependency resolution failed: %w", err)
	}

	al.app.logger.Debug("Module initialization order", "order", moduleOrder)

	// Step 3: Initialize modules and populate service registry
	if err := al.initializeModulesWithServiceRegistry(ctx, moduleOrder); err != nil {
		if emitErr := al.emitLifecycleEvent(ctx, "initialization.failed", map[string]interface{}{
			"error": err.Error(),
			"phase": "module_initialization",
		}); emitErr != nil {
			al.app.logger.Error("Failed to emit initialization failed event", "error", emitErr)
		}
		return fmt.Errorf("module initialization failed: %w", err)
	}

	// Step 4: Register core framework services
	if err := al.registerFrameworkServices(); err != nil {
		if emitErr := al.emitLifecycleEvent(ctx, "initialization.failed", map[string]interface{}{
			"error": err.Error(),
			"phase": "framework_services",
		}); emitErr != nil {
			al.app.logger.Error("Failed to emit initialization failed event", "error", emitErr)
		}
		return fmt.Errorf("framework service registration failed: %w", err)
	}

	// Emit lifecycle event: Initialization completed
	if err := al.emitLifecycleEvent(ctx, "initialization.completed", nil); err != nil {
		al.app.logger.Error("Failed to emit initialization completed event", "error", err)
	}

	return nil
}

// StartWithLifecycle starts the application with deterministic ordering and lifecycle events
func (al *ApplicationLifecycle) StartWithLifecycle(ctx context.Context) error {
	if al.isStarted {
		return ErrApplicationAlreadyStarted
	}

	// Emit lifecycle event: Startup started
	if err := al.emitLifecycleEvent(ctx, "startup.started", nil); err != nil {
		al.app.logger.Error("Failed to emit startup started event", "error", err)
	}

	// Get modules in deterministic start order (same as dependency resolution)
	moduleOrder, err := al.app.resolveDependencies()
	if err != nil {
		if emitErr := al.emitLifecycleEvent(ctx, "startup.failed", map[string]interface{}{
			"error": err.Error(),
			"phase": "dependency_resolution",
		}); emitErr != nil {
			al.app.logger.Error("Failed to emit startup failed event", "error", emitErr)
		}
		return fmt.Errorf("dependency resolution failed during startup: %w", err)
	}

	// Start modules in dependency order with health monitoring
	for _, moduleName := range moduleOrder {
		module := al.app.moduleRegistry[moduleName]

		// Emit per-module startup event
		if err := al.emitLifecycleEvent(ctx, "module.starting", map[string]interface{}{
			"module": moduleName,
		}); err != nil {
			al.app.logger.Error("Failed to emit module starting event", "module", moduleName, "error", err)
		}

		startableModule, ok := module.(Startable)
		if !ok {
			al.app.logger.Debug("Module does not implement Startable, skipping", "module", moduleName)
			continue
		}

		al.app.logger.Info("Starting module", "module", moduleName)
		if err := startableModule.Start(ctx); err != nil {
			if emitErr := al.emitLifecycleEvent(ctx, "startup.failed", map[string]interface{}{
				"error":  err.Error(),
				"module": moduleName,
				"phase":  "module_start",
			}); emitErr != nil {
				al.app.logger.Error("Failed to emit startup failed event", "error", emitErr)
			}
			return fmt.Errorf("failed to start module %s: %w", moduleName, err)
		}

		// Register module health checker if available
		if healthChecker, ok := module.(health.HealthChecker); ok {
			if err := al.healthAggregator.RegisterCheck(ctx, healthChecker); err != nil {
				al.app.logger.Error("Failed to register health checker", "module", moduleName, "error", err)
			} else {
				al.app.logger.Debug("Registered health checker for module", "module", moduleName)
			}
		}

		// Emit per-module started event
		if err := al.emitLifecycleEvent(ctx, "module.started", map[string]interface{}{
			"module": moduleName,
		}); err != nil {
			al.app.logger.Error("Failed to emit module started event", "module", moduleName, "error", err)
		}
	}

	al.isStarted = true

	// Emit lifecycle event: Startup completed
	if err := al.emitLifecycleEvent(ctx, "startup.completed", nil); err != nil {
		al.app.logger.Error("Failed to emit startup completed event", "error", err)
	}

	return nil
}

// StopWithLifecycle stops the application with reverse deterministic ordering and graceful shutdown
func (al *ApplicationLifecycle) StopWithLifecycle(shutdownCtx context.Context) error {
	if !al.isStarted {
		return ErrApplicationNotStarted
	}

	// Use the provided context or create a default timeout context
	var ctx context.Context
	var cancel context.CancelFunc
	if shutdownCtx != nil {
		// Create a derived context with timeout from the provided context
		ctx, cancel = context.WithTimeout(shutdownCtx, al.stopTimeout)
		defer cancel()
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), al.stopTimeout)
		defer cancel()
	}

	// Emit lifecycle event: Shutdown started
	if err := al.emitLifecycleEvent(ctx, "shutdown.started", nil); err != nil {
		al.app.logger.Error("Failed to emit shutdown started event", "error", err)
	}

	// Get modules in reverse deterministic order (reverse dependency order)
	moduleOrder, err := al.app.resolveDependencies()
	if err != nil {
		return fmt.Errorf("dependency resolution failed during shutdown: %w", err)
	}

	// Reverse the order for shutdown
	slices.Reverse(moduleOrder)

	// Stop modules in reverse dependency order
	var lastErr error
	for _, moduleName := range moduleOrder {
		module := al.app.moduleRegistry[moduleName]

		// Emit per-module stopping event
		if err := al.emitLifecycleEvent(ctx, "module.stopping", map[string]interface{}{
			"module": moduleName,
		}); err != nil {
			al.app.logger.Error("Failed to emit module stopping event", "module", moduleName, "error", err)
		}

		stoppableModule, ok := module.(Stoppable)
		if !ok {
			al.app.logger.Debug("Module does not implement Stoppable, skipping", "module", moduleName)
			continue
		}

		al.app.logger.Info("Stopping module", "module", moduleName)
		if err := stoppableModule.Stop(ctx); err != nil {
			al.app.logger.Error("Error stopping module", "module", moduleName, "error", err)
			lastErr = err

			// Emit module stop failed event but continue with other modules
			if emitErr := al.emitLifecycleEvent(ctx, "module.stop_failed", map[string]interface{}{
				"module": moduleName,
				"error":  err.Error(),
			}); emitErr != nil {
				al.app.logger.Error("Failed to emit module stop failed event", "error", emitErr)
			}
		} else {
			// Emit per-module stopped event
			if err := al.emitLifecycleEvent(ctx, "module.stopped", map[string]interface{}{
				"module": moduleName,
			}); err != nil {
				al.app.logger.Error("Failed to emit module stopped event", "module", moduleName, "error", err)
			}
		}
	}

	al.isStarted = false

	// Stop lifecycle dispatcher last
	if err := al.lifecycleDispatcher.Stop(ctx); err != nil {
		al.app.logger.Error("Failed to stop lifecycle dispatcher", "error", err)
		if lastErr == nil {
			lastErr = err
		}
	}

	// Emit lifecycle event: Shutdown completed (if dispatcher is still running)
	if lastErr == nil {
		if err := al.emitLifecycleEvent(ctx, "shutdown.completed", nil); err != nil {
			al.app.logger.Error("Failed to emit shutdown completed event", "error", err)
		}
	} else {
		if emitErr := al.emitLifecycleEvent(ctx, "shutdown.failed", map[string]interface{}{
			"error": lastErr.Error(),
		}); emitErr != nil {
			al.app.logger.Error("Failed to emit shutdown failed event", "error", emitErr)
		}
	}

	return lastErr
}

// loadAndValidateConfiguration loads configuration from all sources and validates it
func (al *ApplicationLifecycle) loadAndValidateConfiguration(ctx context.Context) error {
	// Load application configuration using the new config loader
	if err := al.configLoader.Load(ctx, al.app.ConfigProvider().GetConfig()); err != nil {
		return fmt.Errorf("failed to load application configuration: %w", err)
	}

	// Validate application configuration
	if err := al.configValidator.ValidateStruct(ctx, al.app.ConfigProvider().GetConfig()); err != nil {
		return fmt.Errorf("application configuration validation failed: %w", err)
	}

	// Load and validate module configurations
	for sectionName, provider := range al.app.ConfigSections() {
		al.app.logger.Debug("Loading configuration section", "section", sectionName)

		if err := al.configLoader.Load(ctx, provider.GetConfig()); err != nil {
			return fmt.Errorf("failed to load configuration for section '%s': %w", sectionName, err)
		}

		if err := al.configValidator.ValidateStruct(ctx, provider.GetConfig()); err != nil {
			return fmt.Errorf("configuration validation failed for section '%s': %w", sectionName, err)
		}
	}

	return nil
}

// initializeModulesWithServiceRegistry initializes modules and populates the service registry
func (al *ApplicationLifecycle) initializeModulesWithServiceRegistry(ctx context.Context, moduleOrder []string) error {
	for _, moduleName := range moduleOrder {
		module := al.app.moduleRegistry[moduleName]

		// Inject services if module is service-aware
		if _, ok := module.(ServiceAware); ok {
			var err error
			al.app.moduleRegistry[moduleName], err = al.app.injectServices(module)
			if err != nil {
				return fmt.Errorf("failed to inject services for module '%s': %w", moduleName, err)
			}
			module = al.app.moduleRegistry[moduleName] // Update reference after injection
		}

		// Set current module context for service registration tracking
		if al.app.enhancedSvcRegistry != nil {
			al.app.enhancedSvcRegistry.SetCurrentModule(module)
		}

		// Initialize the module
		err := module.Init(al.app)
		if err != nil {
			return fmt.Errorf("failed to initialize module '%s': %w", moduleName, err)
		}

		al.app.logger.Info("Initialized module", "module", moduleName, "type", fmt.Sprintf("%T", module))

		// Register services provided by the module
		if serviceAware, ok := module.(ServiceAware); ok {
			services := serviceAware.ProvidesServices()
			for _, serviceProvider := range services {
				if err := al.app.RegisterService(serviceProvider.Name, serviceProvider.Instance); err != nil {
					return fmt.Errorf("failed to register service '%s' from module '%s': %w", serviceProvider.Name, moduleName, err)
				}
				al.app.logger.Debug("Registered service", "name", serviceProvider.Name, "module", moduleName)
			}
		}
	}

	return nil
}

// registerFrameworkServices registers core framework services in the registry
func (al *ApplicationLifecycle) registerFrameworkServices() error {
	// Register the enhanced service registry
	if err := al.app.RegisterService("ServiceRegistry", al.serviceRegistry); err != nil {
		return fmt.Errorf("failed to register ServiceRegistry: %w", err)
	}

	// Register the configuration loader
	if err := al.app.RegisterService("ConfigLoader", al.configLoader); err != nil {
		return fmt.Errorf("failed to register ConfigLoader: %w", err)
	}

	// Register the configuration validator
	if err := al.app.RegisterService("ConfigValidator", al.configValidator); err != nil {
		return fmt.Errorf("failed to register ConfigValidator: %w", err)
	}

	// Register the lifecycle event dispatcher
	if err := al.app.RegisterService("LifecycleDispatcher", al.lifecycleDispatcher); err != nil {
		return fmt.Errorf("failed to register LifecycleDispatcher: %w", err)
	}

	// Register the health aggregator
	if err := al.app.RegisterService("HealthAggregator", al.healthAggregator); err != nil {
		return fmt.Errorf("failed to register HealthAggregator: %w", err)
	}

	return nil
}

// emitLifecycleEvent emits a lifecycle event through the dispatcher
func (al *ApplicationLifecycle) emitLifecycleEvent(ctx context.Context, eventType string, metadata map[string]interface{}) error {
	event := &lifecycle.Event{
		Type:      lifecycle.EventType(eventType),
		Timestamp: time.Now(),
		Source:    "application",
		Metadata:  metadata,
		Version:   "1.0",
		Phase:     lifecycle.PhaseUnknown, // Will be set appropriately based on eventType
		Status:    lifecycle.EventStatusCompleted,
	}

	if err := al.lifecycleDispatcher.Dispatch(ctx, event); err != nil {
		return fmt.Errorf("failed to dispatch lifecycle event: %w", err)
	}
	return nil
}

// SetStopTimeout sets the timeout for graceful shutdown
func (al *ApplicationLifecycle) SetStopTimeout(timeout time.Duration) {
	al.stopTimeout = timeout
}

// IsStarted returns whether the application is currently started
func (al *ApplicationLifecycle) IsStarted() bool {
	return al.isStarted
}

// GetHealthAggregator returns the health aggregator for external access
func (al *ApplicationLifecycle) GetHealthAggregator() health.HealthAggregator {
	return al.healthAggregator
}

// GetLifecycleDispatcher returns the lifecycle event dispatcher for external access
func (al *ApplicationLifecycle) GetLifecycleDispatcher() lifecycle.EventDispatcher {
	return al.lifecycleDispatcher
}
