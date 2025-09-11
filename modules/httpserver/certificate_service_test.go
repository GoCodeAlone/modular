package httpserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular"
)

// MockCertificateService implements CertificateService for testing
type MockCertificateService struct {
	certs map[string]*tls.Certificate
}

func NewMockCertificateService() *MockCertificateService {
	return &MockCertificateService{
		certs: make(map[string]*tls.Certificate),
	}
}

func (m *MockCertificateService) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if clientHello == nil || clientHello.ServerName == "" {
		return nil, fmt.Errorf("server name is empty")
	}

	cert, ok := m.certs[clientHello.ServerName]
	if !ok {
		return nil, fmt.Errorf("no certificate found for domain: %s", clientHello.ServerName)
	}

	return cert, nil
}

func (m *MockCertificateService) AddCertificate(domain string, cert *tls.Certificate) {
	m.certs[domain] = cert
}

// SimpleMockApplication is a minimal implementation for the certificate service tests
type SimpleMockApplication struct {
	config     map[string]modular.ConfigProvider
	logger     modular.Logger
	defaultCfg modular.ConfigProvider
}

func NewSimpleMockApplication() *SimpleMockApplication {
	return &SimpleMockApplication{
		config: make(map[string]modular.ConfigProvider),
		logger: NewSimpleMockLogger(),
	}
}

func (m *SimpleMockApplication) RegisterConfigSection(name string, provider modular.ConfigProvider) {
	m.config[name] = provider
	if m.defaultCfg == nil {
		m.defaultCfg = provider
	}
}

func (m *SimpleMockApplication) GetConfigSection(name string) (modular.ConfigProvider, error) {
	cfg, ok := m.config[name]
	if !ok {
		return nil, fmt.Errorf("config section %s not found", name)
	}
	return cfg, nil
}

func (m *SimpleMockApplication) Logger() modular.Logger {
	return m.logger
}

func (m *SimpleMockApplication) SetLogger(logger modular.Logger) {
	m.logger = logger
}

// Implement additional required methods for modular.Application interface
func (m *SimpleMockApplication) ConfigProvider() modular.ConfigProvider {
	return m.defaultCfg
}

func (m *SimpleMockApplication) SvcRegistry() modular.ServiceRegistry {
	return nil // Not needed for these tests
}

func (m *SimpleMockApplication) RegisterModule(_ modular.Module) {
	// No-op for these tests
}

func (m *SimpleMockApplication) ConfigSections() map[string]modular.ConfigProvider {
	return m.config
}

func (m *SimpleMockApplication) RegisterService(_ string, _ interface{}) error {
	return nil // No-op for these tests
}

func (m *SimpleMockApplication) GetService(_ string, _ interface{}) error {
	return nil // No-op for these tests
}

func (m *SimpleMockApplication) Init() error {
	return nil // No-op for these tests
}

func (m *SimpleMockApplication) Start() error {
	return nil // No-op for these tests
}

func (m *SimpleMockApplication) Stop() error {
	return nil // No-op for these tests
}

func (m *SimpleMockApplication) Run() error {
	return nil // No-op for these tests
}

func (m *SimpleMockApplication) IsVerboseConfig() bool {
	return false
}

func (m *SimpleMockApplication) SetVerboseConfig(verbose bool) {
	// No-op for these tests
}

// Newly added methods to satisfy updated Application interface
func (m *SimpleMockApplication) Context() context.Context                       { return context.Background() }
func (m *SimpleMockApplication) GetServicesByModule(moduleName string) []string { return []string{} }
func (m *SimpleMockApplication) GetServiceEntry(serviceName string) (*modular.ServiceRegistryEntry, bool) {
	return nil, false
}
func (m *SimpleMockApplication) GetServicesByInterface(interfaceType reflect.Type) []*modular.ServiceRegistryEntry {
	return []*modular.ServiceRegistryEntry{}
}

// SimpleMockLogger implements modular.Logger for certificate service tests
type SimpleMockLogger struct{}

func NewSimpleMockLogger() *SimpleMockLogger {
	return &SimpleMockLogger{}
}

func (l *SimpleMockLogger) Info(_ string, _ ...interface{})  {}
func (l *SimpleMockLogger) Error(_ string, _ ...interface{}) {}
func (l *SimpleMockLogger) Debug(_ string, _ ...interface{}) {}
func (l *SimpleMockLogger) Warn(_ string, _ ...interface{})  {}
func (l *SimpleMockLogger) Fatal(_ string, _ ...interface{}) {}

// SimpleMockHandler is a simple HTTP handler for certificate service tests
type SimpleMockHandler struct{}

func (h *SimpleMockHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("Hello, world!")); err != nil {
		// Log error but don't fail
		fmt.Printf("Failed to write response: %v\n", err)
	}
}

func TestHTTPServerWithCertificateService(t *testing.T) {
	// Create a mock application
	app := NewSimpleMockApplication()

	// Create HTTP server module
	module := NewHTTPServerModule().(*HTTPServerModule)

	// Setup config provider
	mockConfig := &HTTPServerConfig{
		Host: "127.0.0.1",
		Port: 18443,
	}
	app.config["httpserver"] = NewMockConfigProvider(mockConfig)

	// Initialize module
	if err := module.Init(app); err != nil {
		t.Fatalf("Failed to initialize module: %v", err)
	}

	// Create a test configuration with TLS enabled and certificate service
	module.config.TLS = &TLSConfig{
		Enabled:    true,
		UseService: true,
	}

	// Create a mock certificate service
	certService := NewMockCertificateService()

	// Generate a self-signed certificate for testing
	cert, key, err := module.generateSelfSignedCertificate([]string{"example.com"})
	if err != nil {
		t.Fatalf("Failed to generate certificate: %v", err)
	}

	// Load the certificate
	keypair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		t.Fatalf("Failed to load keypair: %v", err)
	}

	// Add certificate to the mock service
	certService.AddCertificate("example.com", &keypair)

	// Assign the certificate service to the module
	module.certificateService = certService

	// Create a mock HTTP handler
	handler := &SimpleMockHandler{}
	module.handler = handler

	// Manually set the started flag to true for testing
	module.started = true

	// Create a server to simulate that it was started
	module.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", module.config.Host, module.config.Port),
		Handler: handler,
	}

	// Set a context with short timeout for testing
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Clean up
	if err := module.Stop(ctx); err != nil {
		t.Fatalf("Failed to stop module: %v", err)
	}
}

func TestCertificateServiceDependency(t *testing.T) {
	module := NewHTTPServerModule().(*HTTPServerModule)
	services := module.RequiresServices()

	// Should have at least router dependency, plus an optional certificate dependency
	dependencyCount := 0
	var certDep modular.ServiceDependency

	for _, dep := range services {
		if dep.Name == "certificate" {
			certDep = dep
			dependencyCount++
		}
	}

	if dependencyCount == 0 {
		t.Fatalf("Certificate dependency not found")
	}

	// Check certificate dependency
	if certDep.Required || !certDep.MatchByInterface {
		t.Errorf("Certificate dependency not configured correctly, got %+v", certDep)
	}

	// Verify certificate dependency interface type
	expectedInterface := reflect.TypeOf((*CertificateService)(nil)).Elem()
	if certDep.SatisfiesInterface != expectedInterface {
		t.Errorf("Certificate dependency has wrong interface type, got %v, want %v",
			certDep.SatisfiesInterface, expectedInterface)
	}
}

func TestFallbackToFileBasedCerts(t *testing.T) {
	// Create a mock application
	app := NewSimpleMockApplication()

	// Create HTTP server module
	module := NewHTTPServerModule().(*HTTPServerModule)

	// Setup config provider
	mockConfig := &HTTPServerConfig{
		Host:            "127.0.0.1",
		Port:            18444,
		ShutdownTimeout: 5 * time.Second, // Use a shorter shutdown timeout for tests
	}
	app.config["httpserver"] = NewMockConfigProvider(mockConfig)

	// Initialize module
	if err := module.Init(app); err != nil {
		t.Fatalf("Failed to initialize module: %v", err)
	}

	// Configure TLS with files
	certFile, keyFile, err := module.generateSelfSignedCertificate([]string{"localhost"})
	if err != nil {
		t.Fatalf("Failed to generate certificate: %v", err)
	}

	module.config.TLS = &TLSConfig{
		Enabled:  true,
		CertFile: certFile,
		KeyFile:  keyFile,
	}

	// Set handler
	module.handler = &SimpleMockHandler{}

	// Even without a certificate service, the module should still start with file-based certs
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use a different port
	module.config.Port = 18444

	// Start the module in a goroutine
	errCh := make(chan error)
	go func() {
		errCh <- module.Start(ctx)
	}()

	// Check if start was successful
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Failed to start module: %v", err)
		}
	case <-time.After(3 * time.Second):
		// Module started successfully
	}

	// Clean up with a fresh context
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()
	if err := module.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop module: %v", err)
	}
}

func TestAutoGeneratedCerts(t *testing.T) {
	// Create a mock application
	app := NewSimpleMockApplication()

	// Create HTTP server module
	module := NewHTTPServerModule().(*HTTPServerModule)

	// Setup config provider
	mockConfig := &HTTPServerConfig{
		Host:            "127.0.0.1",
		Port:            18445,
		ShutdownTimeout: 5 * time.Second, // Use a shorter shutdown timeout for tests
	}
	app.config["httpserver"] = NewMockConfigProvider(mockConfig)

	// Initialize module
	if err := module.Init(app); err != nil {
		t.Fatalf("Failed to initialize module: %v", err)
	}

	// Configure TLS with auto-generated certs
	module.config.TLS = &TLSConfig{
		Enabled:      true,
		AutoGenerate: true,
		Domains:      []string{"localhost"},
	}

	// Set handler
	module.handler = &SimpleMockHandler{}

	// Use a different port
	module.config.Port = 18445

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start the module in a goroutine
	errCh := make(chan error)
	go func() {
		errCh <- module.Start(ctx)
	}()

	// Check if start was successful
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Failed to start module: %v", err)
		}
	case <-time.After(3 * time.Second):
		// Module started successfully
	}

	// Clean up with a fresh context
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()
	if err := module.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop module: %v", err)
	}
}
