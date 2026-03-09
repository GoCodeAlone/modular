package chimux

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Static errors for bdd_core_test.go
var (
	errFailedToInitializeApp          = errors.New("failed to initialize app")
	errFailedToGetRouterService       = errors.New("failed to get router service")
	errFailedToGetChimuxRouterService = errors.New("failed to get chimux router service")
	errUnderlyingAppNoObservers       = errors.New("underlying application does not support observers")
	errTenantConfigNotFound           = errors.New("tenant config not found")
)

// ChiMux BDD Test Context
type ChiMuxBDDTestContext struct {
	app                 modular.Application
	module              *ChiMuxModule
	routerService       *ChiMuxModule
	chiService          *ChiMuxModule
	config              *ChiMuxConfig
	lastError           error
	testServer          *httptest.Server
	routes              map[string]string
	middlewareProviders []MiddlewareProvider
	routeGroups         []string
	eventObserver       *testEventObserver
	lastResponse        *httptest.ResponseRecorder
	appliedMiddleware   []string // track applied middleware names for removal simulation
}

// Test event observer for capturing emitted events
type testEventObserver struct {
	mu     sync.RWMutex
	events []cloudevents.Event
}

func newTestEventObserver() *testEventObserver {
	return &testEventObserver{
		events: make([]cloudevents.Event, 0),
	}
}

func (t *testEventObserver) OnEvent(ctx context.Context, event cloudevents.Event) error {
	clone := event.Clone()
	t.mu.Lock()
	t.events = append(t.events, clone)
	t.mu.Unlock()
	return nil
}

func (t *testEventObserver) ObserverID() string {
	return "test-observer"
}

func (t *testEventObserver) GetEvents() []cloudevents.Event {
	t.mu.RLock()
	defer t.mu.RUnlock()
	events := make([]cloudevents.Event, len(t.events))
	copy(events, t.events)
	return events
}

func (t *testEventObserver) ClearEvents() {
	t.mu.Lock()
	t.events = make([]cloudevents.Event, 0)
	t.mu.Unlock()
}

func (ctx *ChiMuxBDDTestContext) resetContext() {
	ctx.app = nil
	ctx.module = nil
	ctx.routerService = nil
	ctx.chiService = nil
	ctx.config = nil
	ctx.lastError = nil
	if ctx.testServer != nil {
		ctx.testServer.Close()
		ctx.testServer = nil
	}
	ctx.routes = make(map[string]string)
	ctx.middlewareProviders = []MiddlewareProvider{}
	ctx.routeGroups = []string{}
	ctx.eventObserver = nil
	ctx.appliedMiddleware = []string{}
}

func (ctx *ChiMuxBDDTestContext) iHaveAModularApplicationWithChimuxModuleConfigured() error {
	ctx.resetContext()

	// Create application
	logger := &testLogger{}

	// Create basic chimux configuration for testing
	ctx.config = &ChiMuxConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Accept", "Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           300,
		Timeout:          60 * time.Second,
		BasePath:         "",
	}

	// Create provider with the chimux config
	chimuxConfigProvider := modular.NewStdConfigProvider(ctx.config)

	// Create app with empty main config
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})

	// Create mock tenant application since chimux requires tenant app
	mockTenantApp := &mockTenantApplication{
		Application: modular.NewObservableApplication(mainConfigProvider, logger),
		tenantService: &mockTenantService{
			configs: make(map[modular.TenantID]map[string]modular.ConfigProvider),
		},
	}

	// Create test event observer
	ctx.eventObserver = newTestEventObserver()

	// Register the chimux config section first
	mockTenantApp.RegisterConfigSection("chimux", chimuxConfigProvider)

	// Create and register chimux module
	ctx.module = NewChiMuxModule().(*ChiMuxModule)
	mockTenantApp.RegisterModule(ctx.module)

	// Register observers BEFORE initialization
	if err := ctx.module.RegisterObservers(mockTenantApp); err != nil {
		return fmt.Errorf("failed to register module observers: %w", err)
	}

	// Register our test observer to capture events
	if err := mockTenantApp.RegisterObserver(ctx.eventObserver); err != nil {
		return fmt.Errorf("failed to register test observer: %w", err)
	}

	// Initialize
	if err := mockTenantApp.Init(); err != nil {
		return fmt.Errorf("%w: %w", errFailedToInitializeApp, err)
	}

	ctx.app = mockTenantApp
	return nil
}

func (ctx *ChiMuxBDDTestContext) theChimuxModuleIsInitialized() error {
	// Module should already be initialized in the background step
	return nil
}

func (ctx *ChiMuxBDDTestContext) theRouterServiceShouldBeAvailable() error {
	var routerService *ChiMuxModule
	if err := ctx.app.GetService("router", &routerService); err != nil {
		return fmt.Errorf("%w: %w", errFailedToGetRouterService, err)
	}

	ctx.routerService = routerService
	return nil
}

func (ctx *ChiMuxBDDTestContext) theChiRouterServiceShouldBeAvailable() error {
	var chiService *ChiMuxModule
	if err := ctx.app.GetService("chimux.router", &chiService); err != nil {
		return fmt.Errorf("%w: %w", errFailedToGetChimuxRouterService, err)
	}

	ctx.chiService = chiService
	return nil
}

func (ctx *ChiMuxBDDTestContext) theBasicRouterServiceShouldBeAvailable() error {
	return ctx.theRouterServiceShouldBeAvailable()
}

func (ctx *ChiMuxBDDTestContext) iHaveARouterServiceAvailable() error {
	if ctx.routerService == nil {
		return ctx.theRouterServiceShouldBeAvailable()
	}
	return nil
}

func (ctx *ChiMuxBDDTestContext) iHaveABasicRouterServiceAvailable() error {
	return ctx.iHaveARouterServiceAvailable()
}

func (ctx *ChiMuxBDDTestContext) iHaveAccessToTheChiRouterService() error {
	if ctx.chiService == nil {
		return ctx.theChiRouterServiceShouldBeAvailable()
	}
	return nil
}

// Mock tenant application for testing
type mockTenantApplication struct {
	modular.Application
	tenantService *mockTenantService
}

func (mta *mockTenantApplication) RegisterObserver(observer modular.Observer, eventTypes ...string) error {
	if subject, ok := mta.Application.(modular.Subject); ok {
		if err := subject.RegisterObserver(observer, eventTypes...); err != nil {
			return fmt.Errorf("register observer failed: %w", err)
		}
		return nil
	}
	return errUnderlyingAppNoObservers
}

func (mta *mockTenantApplication) UnregisterObserver(observer modular.Observer) error {
	if subject, ok := mta.Application.(modular.Subject); ok {
		if err := subject.UnregisterObserver(observer); err != nil {
			return fmt.Errorf("unregister observer failed: %w", err)
		}
		return nil
	}
	return errUnderlyingAppNoObservers
}

func (mta *mockTenantApplication) NotifyObservers(ctx context.Context, event cloudevents.Event) error {
	if subject, ok := mta.Application.(modular.Subject); ok {
		if err := subject.NotifyObservers(ctx, event); err != nil {
			return fmt.Errorf("notify observers failed: %w", err)
		}
		return nil
	}
	return errUnderlyingAppNoObservers
}

func (mta *mockTenantApplication) GetObservers() []modular.ObserverInfo {
	if subject, ok := mta.Application.(modular.Subject); ok {
		return subject.GetObservers()
	}
	return []modular.ObserverInfo{}
}

type mockTenantService struct {
	configs map[modular.TenantID]map[string]modular.ConfigProvider
}

func (mts *mockTenantService) GetTenantConfig(tenantID modular.TenantID, section string) (modular.ConfigProvider, error) {
	if tenantConfigs, exists := mts.configs[tenantID]; exists {
		if config, exists := tenantConfigs[section]; exists {
			return config, nil
		}
	}
	return nil, errTenantConfigNotFound
}

func (mts *mockTenantService) GetTenants() []modular.TenantID {
	tenants := make([]modular.TenantID, 0, len(mts.configs))
	for tenantID := range mts.configs {
		tenants = append(tenants, tenantID)
	}
	return tenants
}

func (mts *mockTenantService) RegisterTenant(tenantID modular.TenantID, configs map[string]modular.ConfigProvider) error {
	mts.configs[tenantID] = configs
	return nil
}

func (mts *mockTenantService) RegisterTenantAwareModule(module modular.TenantAwareModule) error {
	// Mock implementation - just return nil
	return nil
}

func (mta *mockTenantApplication) GetTenantService() (modular.TenantService, error) {
	return mta.tenantService, nil
}

func (mta *mockTenantApplication) WithTenant(tenantID modular.TenantID) (*modular.TenantContext, error) {
	return modular.NewTenantContext(context.Background(), tenantID), nil
}

func (mta *mockTenantApplication) GetTenantConfig(tenantID modular.TenantID, section string) (modular.ConfigProvider, error) {
	return mta.tenantService.GetTenantConfig(tenantID, section)
}

// Test logger for BDD tests
type testLogger struct{}

func (l *testLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (l *testLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *testLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *testLogger) Error(msg string, keysAndValues ...interface{}) {}
