package httpserver

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockApplication is a mock implementation of the modular.Application interface
type MockApplication struct {
	mock.Mock
}

func (m *MockApplication) ConfigProvider() modular.ConfigProvider {
	args := m.Called()
	return args.Get(0).(modular.ConfigProvider)
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

func (m *MockApplication) GetConfigSection(name string) (modular.ConfigProvider, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(modular.ConfigProvider), args.Error(1)
}

func (m *MockApplication) SvcRegistry() modular.ServiceRegistry {
	args := m.Called()
	return args.Get(0).(modular.ServiceRegistry)
}

func (m *MockApplication) RegisterModule(module modular.Module) {
	m.Called(module)
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

func (m *MockApplication) Init() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockApplication) Start() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockApplication) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockApplication) Run() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockApplication) IsVerboseConfig() bool {
	return false
}

func (m *MockApplication) SetVerboseConfig(verbose bool) {
	// No-op in mock
}

// MockLogger is a mock implementation of the modular.Logger interface
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, keyvals ...interface{}) {
	args := append([]interface{}{msg}, keyvals...)
	m.Called(args...)
}

func (m *MockLogger) Info(msg string, keyvals ...interface{}) {
	args := append([]interface{}{msg}, keyvals...)
	m.Called(args...)
}

func (m *MockLogger) Warn(msg string, keyvals ...interface{}) {
	args := append([]interface{}{msg}, keyvals...)
	m.Called(args...)
}

func (m *MockLogger) Error(msg string, keyvals ...interface{}) {
	args := append([]interface{}{msg}, keyvals...)
	m.Called(args...)
}

// MockConfigProvider is a mock implementation of the modular.ConfigProvider interface
type MockConfigProvider struct {
	mock.Mock
	config *HTTPServerConfig
}

func (m *MockConfigProvider) GetConfig() interface{} {
	return m.config
}

func NewMockConfigProvider(config *HTTPServerConfig) *MockConfigProvider {
	return &MockConfigProvider{
		config: config,
	}
}

// MockHandler is a simple HTTP handler for testing
type MockHandler struct {
	ResponseStatus int
	ResponseBody   string
}

func (h *MockHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(h.ResponseStatus)
	if _, err := w.Write([]byte(h.ResponseBody)); err != nil {
		// Log error but don't fail the test
		fmt.Printf("Failed to write response: %v\n", err)
	}
}

func TestModuleName(t *testing.T) {
	module := NewHTTPServerModule()
	assert.Equal(t, "httpserver", module.Name())
}

func TestRegisterConfig(t *testing.T) {
	module := NewHTTPServerModule()
	mockApp := new(MockApplication)

	// Mock the GetConfigSection call that checks if config exists
	mockApp.On("GetConfigSection", "httpserver").Return(nil, errors.New("config not found"))
	mockApp.On("RegisterConfigSection", "httpserver", mock.AnythingOfType("*modular.StdConfigProvider")).Return()

	// Use type assertion to call RegisterConfig
	configurable, ok := module.(modular.Configurable)
	assert.True(t, ok, "Module should implement Configurable interface")
	err := configurable.RegisterConfig(mockApp)
	assert.NoError(t, err)
	mockApp.AssertExpectations(t)
}

func TestInit(t *testing.T) {
	module := &HTTPServerModule{}
	mockApp := new(MockApplication)
	mockLogger := new(MockLogger)
	mockConfig := &HTTPServerConfig{
		Host:            "127.0.0.1",
		Port:            8080,
		ReadTimeout:     15,
		WriteTimeout:    15,
		IdleTimeout:     60,
		ShutdownTimeout: 30,
	}
	mockConfigProvider := NewMockConfigProvider(mockConfig)

	mockApp.On("Logger").Return(mockLogger)
	mockLogger.On("Info", "Initializing HTTP server module").Return()
	mockApp.On("GetConfigSection", "httpserver").Return(mockConfigProvider, nil)

	err := module.Init(mockApp)
	assert.NoError(t, err)
	assert.Equal(t, mockConfig, module.config)
	assert.Equal(t, mockLogger, module.logger)
	mockApp.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestConstructor(t *testing.T) {
	module := &HTTPServerModule{}
	mockApp := new(MockApplication)
	mockHandler := &MockHandler{
		ResponseStatus: http.StatusOK,
		ResponseBody:   "Hello, World!",
	}

	constructor := module.Constructor()
	services := map[string]any{
		"router": mockHandler,
	}

	result, err := constructor(mockApp, services)
	assert.NoError(t, err)
	assert.Equal(t, module, result)
	// The handler is now wrapped with request events, so we can't do direct equality
	// Instead, verify that handler is set and is not nil
	assert.NotNil(t, module.handler)
}

func TestConstructorErrors(t *testing.T) {
	module := &HTTPServerModule{}
	mockApp := new(MockApplication)

	constructor := module.Constructor()

	// Test with missing router service
	result, err := constructor(mockApp, map[string]any{})
	assert.Error(t, err)
	assert.Nil(t, result)

	// Test with wrong type for router service
	result, err = constructor(mockApp, map[string]any{"router": "not a handler"})
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestStartStop(t *testing.T) {
	module := &HTTPServerModule{}
	mockApp := new(MockApplication)
	mockLogger := new(MockLogger)
	mockHandler := &MockHandler{
		ResponseStatus: http.StatusOK,
		ResponseBody:   "Hello, World!",
	}

	// Use a random available port for testing
	port := 8090

	config := &HTTPServerConfig{
		Host:            "127.0.0.1",
		Port:            port,
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    15 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 30 * time.Second,
	}

	module.app = mockApp
	module.logger = mockLogger
	module.config = config
	module.handler = mockHandler

	// Set up logger expectations
	mockLogger.On("Info", "Starting HTTP server", "address", fmt.Sprintf("127.0.0.1:%d", port)).Return()
	mockLogger.On("Info", "HTTP server started successfully", "address", fmt.Sprintf("127.0.0.1:%d", port)).Return()
	mockLogger.On("Info", "Stopping HTTP server", "timeout", mock.Anything).Return()
	mockLogger.On("Info", "HTTP server stopped successfully").Return()
	// Expect Debug calls for failed event emissions (when no observer is configured)
	mockLogger.On("Debug", "Failed to emit server started event", "error", mock.AnythingOfType("*errors.errorString")).Return()
	mockLogger.On("Debug", "Failed to emit server stopped event", "error", mock.AnythingOfType("*errors.errorString")).Return()
	// Allow for request event debug calls as well
	mockLogger.On("Debug", "Failed to emit request received event", "error", mock.AnythingOfType("*errors.errorString")).Return().Maybe()
	mockLogger.On("Debug", "Failed to emit request handled event", "error", mock.AnythingOfType("*errors.errorString")).Return().Maybe()

	// Start the server
	ctx := context.Background()
	err := module.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, module.started)

	// Make a test request to the server
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
	if assert.NoError(t, err) && resp != nil {
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				t.Logf("Failed to close response body: %v", closeErr)
			}
		}()

		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "Hello, World!", string(body))
	}

	// Stop the server
	err = module.Stop(ctx)
	assert.NoError(t, err)
	assert.False(t, module.started)

	// Verify expectations
	mockLogger.AssertExpectations(t)
}

func TestStartWithNoHandler(t *testing.T) {
	module := &HTTPServerModule{}
	module.config = &HTTPServerConfig{
		Host: "127.0.0.1",
		Port: 8080,
	}

	err := module.Start(context.Background())
	assert.Error(t, err)
	assert.Equal(t, ErrNoHandler, err)
}

func TestStopWithNoServer(t *testing.T) {
	module := &HTTPServerModule{}

	err := module.Stop(context.Background())
	assert.Error(t, err)
	assert.Equal(t, ErrServerNotStarted, err)
}

func TestRequiresServices(t *testing.T) {
	module := &HTTPServerModule{}
	deps := module.RequiresServices()

	// Should have two dependencies: router (required) and certificate (optional)
	assert.Len(t, deps, 2)

	// Verify router dependency
	routerDep := deps[0]
	assert.Equal(t, "router", routerDep.Name)
	assert.True(t, routerDep.Required)

	// Verify certificate dependency
	certDep := deps[1]
	assert.Equal(t, "certificate", certDep.Name)
	assert.False(t, certDep.Required, "Certificate dependency should be optional")
}

func TestProvidesServices(t *testing.T) {
	module := &HTTPServerModule{}
	services := module.ProvidesServices()

	require.Len(t, services, 1)
	assert.Equal(t, "httpserver", services[0].Name)
	assert.Equal(t, "HTTP server module for handling HTTP requests and providing web services", services[0].Description)
	assert.Equal(t, module, services[0].Instance)
}

func TestTLSSupport(t *testing.T) {
	// Skip this test if we can't create temp files
	tempDir, err := os.MkdirTemp("", "httpserver-test")
	if err != nil {
		t.Skip("Could not create temporary directory:", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			t.Logf("Failed to remove temp directory: %v", removeErr)
		}
	}()

	// Create self-signed certificate for testing
	certFile := filepath.Join(tempDir, "cert.pem")
	keyFile := filepath.Join(tempDir, "key.pem")

	// Generate test certificate (this is a simplified version for testing)
	// In a real test, you would generate proper certificates or use fixtures
	err = generateTestCertificate(certFile, keyFile)
	if err != nil {
		t.Skip("Could not generate test certificate:", err)
	}

	module := &HTTPServerModule{}
	mockApp := new(MockApplication)
	mockLogger := new(MockLogger)
	mockHandler := &MockHandler{
		ResponseStatus: http.StatusOK,
		ResponseBody:   "TLS OK",
	}

	// Use an available port for testing
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("Could not get available port:", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close() // Close immediately to release the port for the server

	config := &HTTPServerConfig{
		Host:            "127.0.0.1",
		Port:            port,
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    15 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		TLS: &TLSConfig{
			Enabled:  true,
			CertFile: certFile,
			KeyFile:  keyFile,
		},
	}

	module.app = mockApp
	module.logger = mockLogger
	module.config = config
	module.handler = mockHandler

	// Set up logger expectations
	mockLogger.On("Info", "Starting HTTP server", "address", fmt.Sprintf("127.0.0.1:%d", port)).Return()
	mockLogger.On("Info", "Using TLS configuration", "cert", certFile, "key", keyFile).Return()
	mockLogger.On("Info", "HTTP server started successfully", "address", fmt.Sprintf("127.0.0.1:%d", port)).Return()
	mockLogger.On("Info", "Stopping HTTP server", "timeout", mock.Anything).Return()
	mockLogger.On("Info", "HTTP server stopped successfully").Return()
	// Expect Debug calls for failed event emissions (when no observer is configured)
	mockLogger.On("Debug", "Failed to emit server started event", "error", mock.AnythingOfType("*errors.errorString")).Return()
	mockLogger.On("Debug", "Failed to emit server stopped event", "error", mock.AnythingOfType("*errors.errorString")).Return()
	mockLogger.On("Debug", "Failed to emit TLS configured event", "error", mock.AnythingOfType("*errors.errorString")).Return()
	// Allow for request event debug calls as well
	mockLogger.On("Debug", "Failed to emit request received event", "error", mock.AnythingOfType("*errors.errorString")).Return().Maybe()
	mockLogger.On("Debug", "Failed to emit request handled event", "error", mock.AnythingOfType("*errors.errorString")).Return().Maybe()

	// Start the server
	ctx := context.Background()
	err = module.Start(ctx)
	require.NoError(t, err)

	// Make a test request to the TLS server with InsecureSkipVerify for the self-signed cert
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Get(fmt.Sprintf("https://127.0.0.1:%d", port))
	if assert.NoError(t, err) && resp != nil {
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				t.Logf("Failed to close response body: %v", closeErr)
			}
		}()

		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "TLS OK", string(body))
	}

	// Stop the server
	err = module.Stop(ctx)
	assert.NoError(t, err)

	// Verify expectations
	mockLogger.AssertExpectations(t)
}

func TestTimeoutConfig(t *testing.T) {
	config := &HTTPServerConfig{
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    20 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 30 * time.Second,
	}

	assert.Equal(t, 15*time.Second, config.ReadTimeout)
	assert.Equal(t, 20*time.Second, config.WriteTimeout)
	assert.Equal(t, 60*time.Second, config.IdleTimeout)
	assert.Equal(t, 30*time.Second, config.ShutdownTimeout)

	// Test with zero value (should use defaults from struct tags or validation)
	configZero := &HTTPServerConfig{}
	err := configZero.Validate()
	assert.NoError(t, err)
	assert.Equal(t, 15*time.Second, configZero.ReadTimeout)
}

// Helper function to generate a self-signed certificate for TLS testing
func generateTestCertificate(certFile, keyFile string) error {
	// Generate a private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create a certificate template
	notBefore := time.Now()
	notAfter := notBefore.Add(24 * time.Hour) // Valid for 24 hours

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test Co"},
			CommonName:   "localhost",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	// Create the certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Write the certificate to file
	certOut, err := os.Create(certFile)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", certFile, err)
	}
	defer func() {
		if err := certOut.Close(); err != nil {
			fmt.Printf("Warning: failed to close cert file: %v\n", err)
		}
	}()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("failed to write data to %s: %w", certFile, err)
	}

	// Write the private key to file
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", keyFile, err)
	}
	defer func() {
		if err := keyOut.Close(); err != nil {
			fmt.Printf("Warning: failed to close key file: %v\n", err)
		}
	}()

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("unable to marshal private key: %w", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("failed to write data to %s: %w", keyFile, err)
	}

	return nil
}
