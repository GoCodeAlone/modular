package modular

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// ApplicationDecorator defines the interface for decorating applications.
// Decorators wrap applications to add additional functionality without
// modifying the core application implementation.
type ApplicationDecorator interface {
	Application

	// GetInnerApplication returns the wrapped application
	GetInnerApplication() Application
}

// ConfigDecorator defines the interface for decorating configuration providers.
// Config decorators can modify, enhance, or validate configuration during loading.
type ConfigDecorator interface {
	// DecorateConfig takes a base config provider and returns a decorated one
	DecorateConfig(base ConfigProvider) ConfigProvider

	// Name returns the decorator name for debugging
	Name() string
}

// BaseApplicationDecorator provides a foundation for application decorators.
// It implements ApplicationDecorator by forwarding all calls to the wrapped application.
type BaseApplicationDecorator struct {
	inner Application
}

// NewBaseApplicationDecorator creates a new base decorator wrapping the given application.
func NewBaseApplicationDecorator(inner Application) *BaseApplicationDecorator {
	return &BaseApplicationDecorator{inner: inner}
}

// GetInnerApplication returns the wrapped application
func (d *BaseApplicationDecorator) GetInnerApplication() Application {
	return d.inner
}

// Forward all Application interface methods to the inner application

func (d *BaseApplicationDecorator) ConfigProvider() ConfigProvider {
	return d.inner.ConfigProvider()
}

func (d *BaseApplicationDecorator) SvcRegistry() ServiceRegistry {
	return d.inner.SvcRegistry()
}

func (d *BaseApplicationDecorator) RegisterModule(module Module) {
	d.inner.RegisterModule(module)
}

func (d *BaseApplicationDecorator) RegisterConfigSection(section string, cp ConfigProvider) {
	d.inner.RegisterConfigSection(section, cp)
}

func (d *BaseApplicationDecorator) ConfigSections() map[string]ConfigProvider {
	return d.inner.ConfigSections()
}

func (d *BaseApplicationDecorator) GetConfigSection(section string) (ConfigProvider, error) {
	return d.inner.GetConfigSection(section) //nolint:wrapcheck // Forwarding call
}

func (d *BaseApplicationDecorator) RegisterService(name string, service any) error {
	return d.inner.RegisterService(name, service) //nolint:wrapcheck // Forwarding call
}

func (d *BaseApplicationDecorator) GetService(name string, target any) error {
	return d.inner.GetService(name, target) //nolint:wrapcheck // Forwarding call
}

func (d *BaseApplicationDecorator) Init() error {
	return d.inner.Init() //nolint:wrapcheck // Forwarding call
}

func (d *BaseApplicationDecorator) Start() error {
	return d.inner.Start() //nolint:wrapcheck // Forwarding call
}

func (d *BaseApplicationDecorator) Stop() error {
	return d.inner.Stop() //nolint:wrapcheck // Forwarding call
}

func (d *BaseApplicationDecorator) Run() error {
	return d.inner.Run() //nolint:wrapcheck // Forwarding call
}

func (d *BaseApplicationDecorator) Logger() Logger {
	return d.inner.Logger()
}

func (d *BaseApplicationDecorator) SetLogger(logger Logger) {
	d.inner.SetLogger(logger)
}

func (d *BaseApplicationDecorator) SetVerboseConfig(enabled bool) {
	d.inner.SetVerboseConfig(enabled)
}

func (d *BaseApplicationDecorator) IsVerboseConfig() bool {
	return d.inner.IsVerboseConfig()
}

// ServiceIntrospector forwards to the inner application's ServiceIntrospector implementation.
func (d *BaseApplicationDecorator) ServiceIntrospector() ServiceIntrospector {
	return d.inner.ServiceIntrospector()
}

// TenantAware methods - if inner supports TenantApplication interface
func (d *BaseApplicationDecorator) GetTenantService() (TenantService, error) {
	if tenantApp, ok := d.inner.(TenantApplication); ok {
		return tenantApp.GetTenantService() //nolint:wrapcheck // Forwarding call
	}
	return nil, ErrServiceNotFound
}

func (d *BaseApplicationDecorator) WithTenant(tenantID TenantID) (*TenantContext, error) {
	if tenantApp, ok := d.inner.(TenantApplication); ok {
		return tenantApp.WithTenant(tenantID) //nolint:wrapcheck // Forwarding call
	}
	return nil, ErrServiceNotFound
}

func (d *BaseApplicationDecorator) GetTenantConfig(tenantID TenantID, section string) (ConfigProvider, error) {
	if tenantApp, ok := d.inner.(TenantApplication); ok {
		return tenantApp.GetTenantConfig(tenantID, section) //nolint:wrapcheck // Forwarding call
	}
	return nil, ErrServiceNotFound
}

// Observer methods - if inner supports Subject interface
func (d *BaseApplicationDecorator) RegisterObserver(observer Observer, eventTypes ...string) error {
	if observableApp, ok := d.inner.(Subject); ok {
		return observableApp.RegisterObserver(observer, eventTypes...) //nolint:wrapcheck // Forwarding call
	}
	return ErrServiceNotFound
}

func (d *BaseApplicationDecorator) UnregisterObserver(observer Observer) error {
	if observableApp, ok := d.inner.(Subject); ok {
		return observableApp.UnregisterObserver(observer) //nolint:wrapcheck // Forwarding call
	}
	return ErrServiceNotFound
}

func (d *BaseApplicationDecorator) NotifyObservers(ctx context.Context, event cloudevents.Event) error {
	if observableApp, ok := d.inner.(Subject); ok {
		return observableApp.NotifyObservers(ctx, event) //nolint:wrapcheck // Forwarding call
	}
	return ErrServiceNotFound
}

func (d *BaseApplicationDecorator) GetObservers() []ObserverInfo {
	if observableApp, ok := d.inner.(Subject); ok {
		return observableApp.GetObservers()
	}
	return nil
}

// RequestReload forwards to the inner application's RequestReload method
func (d *BaseApplicationDecorator) RequestReload(sections ...string) error {
	return d.inner.RequestReload(sections...) //nolint:wrapcheck // Forwarding call
}

// RegisterHealthProvider forwards to the inner application's RegisterHealthProvider method
func (d *BaseApplicationDecorator) RegisterHealthProvider(moduleName string, provider HealthProvider, optional bool) error {
	return d.inner.RegisterHealthProvider(moduleName, provider, optional) //nolint:wrapcheck // Forwarding call
}

// Health forwards to the inner application's Health method
func (d *BaseApplicationDecorator) Health() (HealthAggregator, error) {
	return d.inner.Health() //nolint:wrapcheck // Forwarding call
}
