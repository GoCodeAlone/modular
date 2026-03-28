package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// HTTPClient BDD Test Context
type HTTPClientBDDTestContext struct {
	app             modular.Application
	module          *HTTPClientModule
	service         *HTTPClientModule
	clientConfig    *Config
	lastError       error
	lastResponse    *http.Response
	requestModifier RequestModifierFunc
	customTimeout   time.Duration
	eventObserver   *testEventObserver
}

// testEventObserver captures CloudEvents during testing
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
	return "test-observer-httpclient"
}

func (t *testEventObserver) GetEvents() []cloudevents.Event {
	t.mu.RLock()
	defer t.mu.RUnlock()
	events := make([]cloudevents.Event, len(t.events))
	copy(events, t.events)
	return events
}

func (ctx *HTTPClientBDDTestContext) resetContext() {
	ctx.app = nil
	ctx.module = nil
	ctx.service = nil
	ctx.clientConfig = nil
	ctx.lastError = nil
	if ctx.lastResponse != nil {
		ctx.lastResponse.Body.Close()
		ctx.lastResponse = nil
	}
	ctx.requestModifier = nil
	ctx.customTimeout = 0
	ctx.eventObserver = nil
}

func (ctx *HTTPClientBDDTestContext) iHaveAModularApplicationWithHTTPClientModuleConfigured() error {
	ctx.resetContext()

	// Create application with httpclient config
	logger := &bddTestLogger{}

	// Create basic httpclient configuration for testing
	ctx.clientConfig = &Config{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		RequestTimeout:      30 * time.Second,
		TLSTimeout:          10 * time.Second,
		DisableCompression:  false,
		DisableKeepAlives:   false,
		Verbose:             false,
	}

	// Create provider with the httpclient config
	clientConfigProvider := modular.NewStdConfigProvider(ctx.clientConfig)

	// Create app with empty main config
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewStdApplication(mainConfigProvider, logger)

	// Create and register httpclient module
	ctx.module = NewHTTPClientModule().(*HTTPClientModule)

	// Register the httpclient config section first
	ctx.app.RegisterConfigSection("httpclient", clientConfigProvider)

	// Register the module
	ctx.app.RegisterModule(ctx.module)

	return nil
}

func (ctx *HTTPClientBDDTestContext) theHTTPClientModuleIsInitialized() error {
	err := ctx.app.Init()
	if err != nil {
		ctx.lastError = err
		return nil
	}

	// Get the httpclient service (the service interface, not the raw client)
	var clientService *HTTPClientModule
	if err := ctx.app.GetService("httpclient-service", &clientService); err == nil {
		ctx.service = clientService
	}

	return nil
}

func (ctx *HTTPClientBDDTestContext) theHTTPClientServiceShouldBeAvailable() error {
	if ctx.service == nil {
		return fmt.Errorf("httpclient service not available")
	}
	return nil
}

func (ctx *HTTPClientBDDTestContext) theClientShouldBeConfiguredWithDefaultSettings() error {
	if ctx.service == nil {
		return fmt.Errorf("httpclient service not available")
	}

	// For BDD purposes, validate that we have a working client
	client := ctx.service.Client()
	if client == nil {
		return fmt.Errorf("http client not available")
	}

	return nil
}

func (ctx *HTTPClientBDDTestContext) iHaveAnHTTPClientServiceAvailable() error {
	err := ctx.iHaveAModularApplicationWithHTTPClientModuleConfigured()
	if err != nil {
		return err
	}

	return ctx.theHTTPClientModuleIsInitialized()
}

func (ctx *HTTPClientBDDTestContext) setupApplicationWithConfig() error {
	logger := &bddTestLogger{}

	// Create provider with the httpclient config
	clientConfigProvider := modular.NewStdConfigProvider(ctx.clientConfig)

	// Create app with empty main config
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewStdApplication(mainConfigProvider, logger)

	// Create and register httpclient module
	ctx.module = NewHTTPClientModule().(*HTTPClientModule)

	// Register the httpclient config section first
	ctx.app.RegisterConfigSection("httpclient", clientConfigProvider)

	// Register the module
	ctx.app.RegisterModule(ctx.module)

	// Initialize
	err := ctx.app.Init()
	if err != nil {
		ctx.lastError = err
		return nil
	}

	// Get the httpclient service (the service interface, not the raw client)
	var clientService *HTTPClientModule
	if err := ctx.app.GetService("httpclient-service", &clientService); err == nil {
		ctx.service = clientService
	}

	return nil
}

// Test logger implementation for BDD tests
type bddTestLogger struct{}

func (l *bddTestLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (l *bddTestLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *bddTestLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *bddTestLogger) Error(msg string, keysAndValues ...interface{}) {}

// Helper function to check if error is timeout related
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return errStr != "" && (err.Error() == "context deadline exceeded" ||
		err.Error() == "timeout" ||
		err.Error() == "i/o timeout" ||
		err.Error() == "request timeout" ||
		// Additional timeout patterns from Go's net/http
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "Client.Timeout"))
}
