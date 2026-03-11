package modular

import (
	"context"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Option represents a functional option for configuring applications
type Option func(*ApplicationBuilder) error

// ApplicationBuilder helps construct applications with various decorators and options
type ApplicationBuilder struct {
	baseApp           Application
	logger            Logger
	configProvider    ConfigProvider
	modules           []Module
	configDecorators  []ConfigDecorator
	observers         []ObserverFunc
	tenantLoader      TenantLoader
	enableObserver    bool
	enableTenant      bool
	configLoadedHooks []func(Application) error // Hooks to run after config loading
	tenantGuard       *StandardTenantGuard
	tenantGuardConfig *TenantGuardConfig
	dependencyHints   []DependencyEdge
	drainTimeout      time.Duration
	parallelInit      bool
	dynamicReload     bool
	plugins           []Plugin
}

// ObserverFunc is a functional observer that can be registered with the application
type ObserverFunc func(ctx context.Context, event cloudevents.Event) error

// NewApplication creates a new application with the provided options.
// This is the main entry point for the new builder API.
func NewApplication(opts ...Option) (Application, error) {
	builder := &ApplicationBuilder{
		modules:           make([]Module, 0),
		configDecorators:  make([]ConfigDecorator, 0),
		observers:         make([]ObserverFunc, 0),
		configLoadedHooks: make([]func(Application) error, 0),
	}

	// Apply all options
	for _, opt := range opts {
		if err := opt(builder); err != nil {
			return nil, err
		}
	}

	// Build the application
	return builder.Build()
}

// Build constructs the final application with all decorators applied
func (b *ApplicationBuilder) Build() (Application, error) {
	var app Application

	// Start with base application or create default
	if b.baseApp != nil {
		app = b.baseApp
	} else {
		// Create default config provider if none specified
		if b.configProvider == nil {
			b.configProvider = NewStdConfigProvider(&struct{}{})
		}

		// Create default logger if none specified
		if b.logger == nil {
			return nil, ErrLoggerNotSet
		}

		// Create base application
		if b.enableObserver {
			app = NewObservableApplication(b.configProvider, b.logger)
		} else {
			app = NewStdApplication(b.configProvider, b.logger)
		}
	}

	// Apply config decorators to the base config provider
	if len(b.configDecorators) > 0 {
		decoratedProvider := b.configProvider
		for _, decorator := range b.configDecorators {
			decoratedProvider = decorator.DecorateConfig(decoratedProvider)
		}

		// Update the application's config provider if possible
		if baseApp, ok := app.(*StdApplication); ok {
			baseApp.cfgProvider = decoratedProvider
		} else if obsApp, ok := app.(*ObservableApplication); ok {
			obsApp.cfgProvider = decoratedProvider
		}
	}

	// Apply decorators
	if b.enableTenant && b.tenantLoader != nil {
		app = NewTenantAwareDecorator(app, b.tenantLoader)
	}

	if b.enableObserver && len(b.observers) > 0 {
		app = NewObservableDecorator(app, b.observers...)
	}

	// Create and register tenant guard if configured.
	// Use RegisterService so that the EnhancedServiceRegistry (if enabled) tracks
	// the entry and subsequent RegisterService calls don't overwrite it.
	if b.tenantGuardConfig != nil {
		b.tenantGuard = NewStandardTenantGuard(*b.tenantGuardConfig)
		if err := app.RegisterService("tenant.guard", b.tenantGuard); err != nil {
			return nil, fmt.Errorf("failed to register tenant guard service: %w", err)
		}
	}

	// Unwrap decorators to find the underlying StdApplication.
	baseApp := app
	for {
		if dec, ok := baseApp.(ApplicationDecorator); ok {
			if inner := dec.GetInnerApplication(); inner != nil {
				baseApp = inner
				continue
			}
		}
		break
	}

	// Propagate config-driven dependency hints
	if len(b.dependencyHints) > 0 {
		if stdApp, ok := baseApp.(*StdApplication); ok {
			stdApp.dependencyHints = b.dependencyHints
		} else if obsApp, ok := baseApp.(*ObservableApplication); ok {
			obsApp.dependencyHints = b.dependencyHints
		}
	}

	// Propagate drain timeout
	if b.drainTimeout > 0 {
		if stdApp, ok := baseApp.(*StdApplication); ok {
			stdApp.drainTimeout = b.drainTimeout
		} else if obsApp, ok := baseApp.(*ObservableApplication); ok {
			obsApp.drainTimeout = b.drainTimeout
		}
	}

	// Propagate dynamic reload
	if b.dynamicReload {
		if stdApp, ok := baseApp.(*StdApplication); ok {
			stdApp.dynamicReload = true
		} else if obsApp, ok := baseApp.(*ObservableApplication); ok {
			obsApp.dynamicReload = true
		}
	}

	// Propagate parallel init
	if b.parallelInit {
		if stdApp, ok := baseApp.(*StdApplication); ok {
			stdApp.parallelInit = true
		} else if obsApp, ok := baseApp.(*ObservableApplication); ok {
			obsApp.parallelInit = true
		}
	}

	// Process plugins
	for _, plugin := range b.plugins {
		for _, mod := range plugin.Modules() {
			app.RegisterModule(mod)
		}
		if withSvc, ok := plugin.(PluginWithServices); ok {
			for _, svcDef := range withSvc.Services() {
				if err := app.RegisterService(svcDef.Name, svcDef.Service); err != nil {
					return nil, fmt.Errorf("plugin %q service %q: %w", plugin.Name(), svcDef.Name, err)
				}
			}
		}
		if withHooks, ok := plugin.(PluginWithHooks); ok {
			for _, hook := range withHooks.InitHooks() {
				app.OnConfigLoaded(hook)
			}
		}
	}

	// Register modules
	for _, module := range b.modules {
		app.RegisterModule(module)
	}

	// Register config loaded hooks
	for _, hook := range b.configLoadedHooks {
		app.OnConfigLoaded(hook)
	}

	return app, nil
}

// WithBaseApplication sets the base application to decorate
func WithBaseApplication(base Application) Option {
	return func(b *ApplicationBuilder) error {
		b.baseApp = base
		return nil
	}
}

// WithLogger sets the logger for the application
func WithLogger(logger Logger) Option {
	return func(b *ApplicationBuilder) error {
		b.logger = logger
		return nil
	}
}

// WithConfigProvider sets the configuration provider
func WithConfigProvider(provider ConfigProvider) Option {
	return func(b *ApplicationBuilder) error {
		b.configProvider = provider
		return nil
	}
}

// WithModules adds modules to the application
func WithModules(modules ...Module) Option {
	return func(b *ApplicationBuilder) error {
		b.modules = append(b.modules, modules...)
		return nil
	}
}

// WithModuleDependency declares that module `from` depends on module `to`,
// injecting an edge into the dependency graph before resolution.
func WithModuleDependency(from, to string) Option {
	return func(b *ApplicationBuilder) error {
		b.dependencyHints = append(b.dependencyHints, DependencyEdge{
			From: from,
			To:   to,
			Type: EdgeTypeModule,
		})
		return nil
	}
}

// WithDrainTimeout sets the timeout for the pre-stop drain phase during shutdown.
func WithDrainTimeout(d time.Duration) Option {
	return func(b *ApplicationBuilder) error {
		b.drainTimeout = d
		return nil
	}
}

// WithParallelInit enables concurrent module initialization at the same topological depth.
func WithParallelInit() Option {
	return func(b *ApplicationBuilder) error {
		b.parallelInit = true
		return nil
	}
}

// WithDynamicReload enables the ReloadOrchestrator, which coordinates
// configuration reloading across all registered Reloadable modules.
func WithDynamicReload() Option {
	return func(b *ApplicationBuilder) error {
		b.dynamicReload = true
		return nil
	}
}

// WithPlugins adds plugins to the application. Each plugin's modules, services,
// and init hooks are registered during Build().
func WithPlugins(plugins ...Plugin) Option {
	return func(b *ApplicationBuilder) error {
		b.plugins = append(b.plugins, plugins...)
		return nil
	}
}

// WithConfigDecorators adds configuration decorators
func WithConfigDecorators(decorators ...ConfigDecorator) Option {
	return func(b *ApplicationBuilder) error {
		b.configDecorators = append(b.configDecorators, decorators...)
		return nil
	}
}

// WithObserver enables observer pattern and adds observer functions
func WithObserver(observers ...ObserverFunc) Option {
	return func(b *ApplicationBuilder) error {
		b.enableObserver = true
		b.observers = append(b.observers, observers...)
		return nil
	}
}

// WithTenantAware enables tenant-aware functionality with the provided loader
func WithTenantAware(loader TenantLoader) Option {
	return func(b *ApplicationBuilder) error {
		b.enableTenant = true
		b.tenantLoader = loader
		return nil
	}
}

// WithOnConfigLoaded registers hooks to run after config loading but before module initialization.
// This is useful for reconfiguring dependencies (logger, metrics, tracing) based on loaded config.
// Multiple hooks can be registered and will be executed in registration order.
//
// Example:
//
//	app, err := modular.NewApplication(
//	    modular.WithLogger(defaultLogger),
//	    modular.WithConfigProvider(configProvider),
//	    modular.WithOnConfigLoaded(func(app modular.Application) error {
//	        config := app.ConfigProvider().GetConfig().(*AppConfig)
//	        if config.LogFormat == "json" {
//	            newLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	            app.SetLogger(newLogger)
//	        }
//	        return nil
//	    }),
//	    modular.WithModules(modules...),
//	)
func WithOnConfigLoaded(hooks ...func(Application) error) Option {
	return func(b *ApplicationBuilder) error {
		b.configLoadedHooks = append(b.configLoadedHooks, hooks...)
		return nil
	}
}

// WithTenantGuardMode enables the tenant guard with the specified mode using default config.
func WithTenantGuardMode(mode TenantGuardMode) Option {
	return func(b *ApplicationBuilder) error {
		if b.tenantGuardConfig == nil {
			cfg := DefaultTenantGuardConfig()
			b.tenantGuardConfig = &cfg
		}
		b.tenantGuardConfig.Mode = mode
		return nil
	}
}

// WithTenantGuardConfig enables the tenant guard with a full configuration.
func WithTenantGuardConfig(config TenantGuardConfig) Option {
	return func(b *ApplicationBuilder) error {
		b.tenantGuardConfig = &config
		return nil
	}
}

// Convenience functions for creating common decorators

// InstanceAwareConfig creates an instance-aware configuration decorator
func InstanceAwareConfig() ConfigDecorator {
	return &instanceAwareConfigDecorator{}
}

// TenantAwareConfigDecorator creates a tenant-aware configuration decorator
func TenantAwareConfigDecorator(loader TenantLoader) ConfigDecorator {
	return &tenantAwareConfigDecorator{loader: loader}
}
