package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockApplication implements modular.Application interface for testing
type MockApplication struct {
	mock.Mock
}

func (m *MockApplication) GetConfigSection(name string) (modular.ConfigProvider, error) {
	args := m.Called(name)
	return args.Get(0).(modular.ConfigProvider), args.Error(1)
}

func (m *MockApplication) RegisterConfigSection(name string, provider modular.ConfigProvider) {
	m.Called(name, provider)
}

func (m *MockApplication) Logger() modular.Logger {
	args := m.Called()
	return args.Get(0).(modular.Logger)
}

func (m *MockApplication) SetLogger(logger modular.Logger) {
	m.Called(logger)
}

func (m *MockApplication) ConfigProvider() modular.ConfigProvider {
	args := m.Called()
	return args.Get(0).(modular.ConfigProvider)
}

func (m *MockApplication) SvcRegistry() modular.ServiceRegistry {
	args := m.Called()
	return args.Get(0).(modular.ServiceRegistry)
}

func (m *MockApplication) ConfigSections() map[string]modular.ConfigProvider {
	args := m.Called()
	return args.Get(0).(map[string]modular.ConfigProvider)
}

func (m *MockApplication) RegisterService(name string, service any) error {
	args := m.Called(name, service)
	return args.Error(0)
}

func (m *MockApplication) GetService(name string, target any) error {
	args := m.Called(name, target)
	return args.Error(0)
}

// Add other required methods to satisfy the interface
func (m *MockApplication) Name() string                                        { return "mock-app" }
func (m *MockApplication) IsInitializing() bool                                { return false }
func (m *MockApplication) IsStarting() bool                                    { return false }
func (m *MockApplication) IsStopping() bool                                    { return false }
func (m *MockApplication) RegisterModule(module modular.Module)                {}
func (m *MockApplication) GetModuleByName(name string) (modular.Module, error) { return nil, nil }
func (m *MockApplication) GetAllModules() []modular.Module                     { return nil }
func (m *MockApplication) Run() error                                          { return nil }
func (m *MockApplication) Shutdown(ctx context.Context) error                  { return nil }
func (m *MockApplication) Init() error                                         { return nil }
func (m *MockApplication) Start() error                                        { return nil }
func (m *MockApplication) Stop() error                                         { return nil }

// MockLogger implements modular.Logger interface for testing
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, keyvals ...interface{}) {
	m.Called(msg, keyvals)
}

func (m *MockLogger) Info(msg string, keyvals ...interface{}) {
	m.Called(msg, keyvals)
}

func (m *MockLogger) Warn(msg string, keyvals ...interface{}) {
	m.Called(msg, keyvals)
}

func (m *MockLogger) Error(msg string, keyvals ...interface{}) {
	m.Called(msg, keyvals)
}

// MockConfigProvider implements modular.ConfigProvider interface for testing
type MockConfigProvider struct {
	mock.Mock
}

func (m *MockConfigProvider) GetConfig() interface{} {
	args := m.Called()
	return args.Get(0)
}

// TestNewHTTPClientModule tests the creation of a new HTTP client module
func TestNewHTTPClientModule(t *testing.T) {
	module := NewHTTPClientModule()
	assert.NotNil(t, module, "Module should not be nil")
	assert.Equal(t, "httpclient", module.Name(), "Module name should be 'httpclient'")
}

// TestHTTPClientModule_Init tests the initialization of the HTTP client module
func TestHTTPClientModule_Init(t *testing.T) {
	// Create mocks
	mockApp := new(MockApplication)
	mockLogger := new(MockLogger)
	mockConfigProvider := new(MockConfigProvider)

	// Setup expectations
	mockApp.On("Logger").Return(mockLogger)
	mockLogger.On("Info", "Initializing HTTP client module", mock.Anything).Return()
	mockApp.On("GetConfigSection", "httpclient").Return(mockConfigProvider, nil)

	// Setup config
	config := &Config{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90,
		RequestTimeout:      30,
		TLSTimeout:          10,
		DisableCompression:  false,
		DisableKeepAlives:   false,
		Verbose:             false,
	}
	mockConfigProvider.On("GetConfig").Return(config)

	// Create and initialize module
	module := NewHTTPClientModule().(*HTTPClientModule)
	err := module.Init(mockApp)

	// Assertions
	assert.NoError(t, err, "Init should not return an error")
	assert.NotNil(t, module.httpClient, "HTTP client should not be nil")
	assert.Equal(t, 30*time.Second, module.httpClient.Timeout, "Timeout should be set correctly")

	// Verify expectations
	mockApp.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
	mockConfigProvider.AssertExpectations(t)
}

// TestHTTPClientModule_Client tests the Client method
func TestHTTPClientModule_Client(t *testing.T) {
	// Create module and manually set the HTTP client
	module := &HTTPClientModule{
		httpClient: &http.Client{},
	}

	client := module.Client()
	assert.NotNil(t, client, "Client should not be nil")
	assert.Equal(t, module.httpClient, client, "Client() should return the module's HTTP client")
}

// TestHTTPClientModule_WithTimeout tests the WithTimeout method
func TestHTTPClientModule_WithTimeout(t *testing.T) {
	// Create module and manually set the HTTP client
	module := &HTTPClientModule{
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{},
		},
	}

	// Test with a positive timeout
	client := module.WithTimeout(60)
	assert.NotNil(t, client, "Client should not be nil")
	assert.Equal(t, 60*time.Second, client.Timeout, "Timeout should be 60 seconds")

	// Test with a negative timeout (should return default client)
	client = module.WithTimeout(-1)
	assert.NotNil(t, client, "Client should not be nil")
	assert.Equal(t, module.httpClient, client, "Should return the default client")
}

// TestHTTPClientModule_RequestModifier tests the request modifier functionality
func TestHTTPClientModule_RequestModifier(t *testing.T) {
	// Create module
	module := &HTTPClientModule{
		modifier: func(r *http.Request) *http.Request {
			r.Header.Set("X-Test", "test-value")
			return r
		},
	}

	// Create a test request
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	// Apply the modifier
	modifiedReq := module.RequestModifier()(req)

	// Verify the modification
	assert.Equal(t, "test-value", modifiedReq.Header.Get("X-Test"), "Header should be set by modifier")
}

// TestHTTPClientModule_SetRequestModifier tests setting a request modifier
func TestHTTPClientModule_SetRequestModifier(t *testing.T) {
	// Create module with default modifier
	module := &HTTPClientModule{
		modifier: func(r *http.Request) *http.Request { return r },
	}

	// Set a new modifier
	module.SetRequestModifier(func(r *http.Request) *http.Request {
		r.Header.Set("X-Custom", "custom-value")
		return r
	})

	// Create a test request
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	// Apply the modifier
	modifiedReq := module.modifier(req)

	// Verify the modification
	assert.Equal(t, "custom-value", modifiedReq.Header.Get("X-Custom"), "Header should be set by new modifier")
}

// TestHTTPClientModule_LoggingTransport tests the logging transport
func TestHTTPClientModule_LoggingTransport(t *testing.T) {
	// Create mocks
	mockLogger := new(MockLogger)

	// Setup temporary file for logging
	tmpDir, err := os.MkdirTemp("", "httpclient-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			t.Logf("Failed to remove temp directory: %v", removeErr)
		}
	}()

	fileLogger, err := NewFileLogger(tmpDir, mockLogger)
	assert.NoError(t, err)

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("Hello, world!")); err != nil {
			t.Logf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	// Create logging transport
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

	transport := &loggingTransport{
		Transport:      http.DefaultTransport,
		Logger:         mockLogger,
		FileLogger:     fileLogger,
		LogHeaders:     true,
		LogBody:        true,
		MaxBodyLogSize: 1024,
		LogToFile:      true,
	}

	// Create client with logging transport
	client := &http.Client{
		Transport: transport,
	}

	// Make a request
	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(req)

	// Assertions
	assert.NoError(t, err, "Request should not fail")
	assert.NotNil(t, resp, "Response should not be nil")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Status code should be 200")

	// Verify logger expectations
	mockLogger.AssertExpectations(t)

	// Cleanup
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Logf("Failed to close response body: %v", closeErr)
	}
	if closeErr := fileLogger.Close(); closeErr != nil {
		t.Logf("Failed to close file logger: %v", closeErr)
	}
}

// TestHTTPClientModule_IntegrationWithServer tests the HTTP client talking to a test server
func TestHTTPClientModule_IntegrationWithServer(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request headers
		assert.Equal(t, "test-value", r.Header.Get("X-Test-Header"), "Header should be sent")

		// Return a response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"success": true}`)); err != nil {
			t.Logf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	// Create module with custom modifier
	module := &HTTPClientModule{
		httpClient: &http.Client{},
		modifier: func(r *http.Request) *http.Request {
			r.Header.Set("X-Test-Header", "test-value")
			return r
		},
	}

	// Create request
	req, _ := http.NewRequest("GET", server.URL, nil)

	// Apply modifier and make the request
	req = module.RequestModifier()(req)
	resp, err := module.Client().Do(req)

	// Assertions
	assert.NoError(t, err, "Request should not fail")
	assert.NotNil(t, resp, "Response should not be nil")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Status code should be 200")
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "Content-Type should be application/json")

	// Cleanup
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Logf("Failed to close response body: %v", closeErr)
	}
}
