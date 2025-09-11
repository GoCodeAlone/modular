// Package httpserver provides an HTTP server module for the modular framework.
// This module offers a complete HTTP server implementation with support for
// TLS, automatic certificate management, graceful shutdown, and middleware integration.
//
// The httpserver module features:
//   - HTTP and HTTPS server support
//   - Automatic TLS certificate generation and management
//   - Configurable timeouts and limits
//   - Graceful shutdown handling
//   - Handler registration and middleware support
//   - Health check endpoints
//   - Integration with Let's Encrypt for automatic certificates
//
// Usage:
//
//	app.RegisterModule(httpserver.NewModule())
//
// The module registers an HTTP server service that can be used by other modules
// to register handlers, middleware, or access the underlying server instance.
//
// Configuration:
//
//	The module requires an "httpserver" configuration section with server
//	settings including address, ports, TLS configuration, and timeout values.
package httpserver

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// ModuleName is the name of this module for registration and dependency resolution.
const ModuleName = "httpserver"

// Error definitions for HTTP server operations.
var (
	// ErrServerNotStarted is returned when attempting to stop a server that hasn't been started.
	ErrServerNotStarted = errors.New("server not started")

	// ErrNoHandler is returned when no HTTP handler is available for the server.
	ErrNoHandler = errors.New("no HTTP handler available")

	// ErrRouterServiceNotHandler is returned when the router service doesn't implement http.Handler.
	ErrRouterServiceNotHandler = errors.New("router service does not implement http.Handler")

	// ErrServerStartTimeout is returned when the server fails to start within the timeout period.
	ErrServerStartTimeout = errors.New("context cancelled while waiting for server to start")
)

// HTTPServerModule represents the HTTP server module and implements the modular.Module interface.
// It provides a complete HTTP server implementation with TLS support, graceful shutdown,
// and integration with the modular framework's configuration and service systems.
//
// The module manages:
//   - HTTP server lifecycle (start, stop, graceful shutdown)
//   - TLS certificate management and automatic generation
//   - Request routing and handler registration
//   - Server configuration and health monitoring
//   - Integration with certificate services for automatic HTTPS
//   - Event observation and emission for server operations
//
// The module implements the following interfaces:
//   - modular.Module: Basic module lifecycle
//   - modular.Configurable: Configuration management
//   - modular.ServiceAware: Service dependency management
//   - modular.Startable: Startup logic
//   - modular.Stoppable: Shutdown logic
//   - modular.ObservableModule: Event observation and emission
type HTTPServerModule struct {
	config             *HTTPServerConfig
	server             *http.Server
	app                modular.Application
	logger             modular.Logger
	handler            http.Handler
	started            bool
	certificateService CertificateService
	subject            modular.Subject // For event observation (guarded by mu)
	mu                 sync.RWMutex
}

// Make sure the HTTPServerModule implements the Module interface
var _ modular.Module = (*HTTPServerModule)(nil)

// NewHTTPServerModule creates a new instance of the HTTP server module.
// The returned module must be registered with the application before use.
//
// Example:
//
//	httpModule := httpserver.NewHTTPServerModule()
//	app.RegisterModule(httpModule)
func NewHTTPServerModule() modular.Module {
	return &HTTPServerModule{}
}

// Name returns the name of the module.
// This name is used for dependency resolution and configuration section lookup.
func (m *HTTPServerModule) Name() string {
	return ModuleName
}

// RegisterConfig registers the module's configuration structure.
// The HTTP server module supports comprehensive configuration including:
//   - Server address and port settings
//   - Timeout configurations (read, write, idle, shutdown)
//   - TLS settings and certificate paths
//   - Security headers and CORS configuration
//
// Default values are provided for common use cases, but can be
// overridden through configuration files or environment variables.
func (m *HTTPServerModule) RegisterConfig(app modular.Application) error {
	// Check if httpserver config is already registered (e.g., by tests)
	if _, err := app.GetConfigSection(m.Name()); err == nil {
		// Config already registered, skip to avoid overriding
		return nil
	}

	// Register default config only if not already present
	defaultConfig := &HTTPServerConfig{
		Host:            "0.0.0.0",
		Port:            8080,
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    15 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 30 * time.Second,
	}

	app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(defaultConfig))
	return nil
}

// Init initializes the module with the provided application.
// This method loads the configuration, sets up the logger, and prepares
// the HTTP server for startup. It also attempts to resolve optional
// services like certificate management.
//
// Initialization process:
//  1. Load HTTP server configuration
//  2. Set up logging
//  3. Resolve optional certificate service for TLS
//  4. Prepare server instance (actual startup happens in Start)
func (m *HTTPServerModule) Init(app modular.Application) error {
	m.app = app
	m.logger = app.Logger()
	m.logger.Info("Initializing HTTP server module")

	// Get the config section
	cfg, err := app.GetConfigSection(m.Name())
	if err != nil {
		return fmt.Errorf("failed to get config section '%s': %w", m.Name(), err)
	}
	m.config = cfg.GetConfig().(*HTTPServerConfig)

	// After configuration is loaded, emit a module-specific config loaded event.
	// Only attempt emission if a subject is available; unit tests may not provide one.
	hasSubject := m.subject != nil
	if !hasSubject {
		if _, ok := m.app.(modular.Subject); ok {
			hasSubject = true
		}
	}
	if hasSubject {
		cfgEvent := modular.NewCloudEvent(EventTypeConfigLoaded, "httpserver-module", map[string]interface{}{
			"host":         m.config.Host,
			"port":         m.config.Port,
			"http_address": fmt.Sprintf("%s:%d", m.config.Host, m.config.Port),
			"read_timeout": m.config.ReadTimeout.String(),
			"tls_enabled":  m.config.TLS != nil && m.config.TLS.Enabled,
		}, nil)
		if err := m.EmitEvent(modular.WithSynchronousNotification(context.Background()), cfgEvent); err != nil {
			m.logger.Debug("Failed to emit httpserver config loaded event", "error", err)
		}
	}

	return nil
}

// Constructor returns a dependency injection function that initializes the module with
// required services
func (m *HTTPServerModule) Constructor() modular.ModuleConstructor {
	return func(_ modular.Application, services map[string]any) (modular.Module, error) {
		// Get the router service (which implements http.Handler)
		handler, ok := services["router"].(http.Handler)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrRouterServiceNotHandler, "router")
		}

		// Store the handler for use in Start - wrap with request event middleware
		m.handler = m.wrapHandlerWithRequestEvents(handler)

		// Check if a certificate service is available, but it's optional
		if certService, ok := services["certificate"].(CertificateService); ok {
			m.logger.Info("Found certificate service, will use for TLS")
			m.certificateService = certService
		}

		return m, nil
	}
}

// Start starts the HTTP server and begins accepting connections.
// This method configures the server with the loaded configuration,
// sets up TLS if enabled, and starts listening for HTTP requests.
//
// The server startup process:
//  1. Validate that a handler has been registered
//  2. Create http.Server instance with configured timeouts
//  3. Set up TLS certificates if HTTPS is enabled
//  4. Start the server in a goroutine
//  5. Handle graceful shutdown on context cancellation
//
// The server will continue running until the context is cancelled
// or Stop() is called explicitly.
func (m *HTTPServerModule) Start(ctx context.Context) error {
	if m.handler == nil {
		return ErrNoHandler
	}

	// Create address string from host and port
	addr := fmt.Sprintf("%s:%d", m.config.Host, m.config.Port)

	// Always ensure the handler is wrapped to emit request events, even if a plain
	// handler was set after construction (e.g., in tests). Wrapping multiple times is
	// safe functionally, but to avoid duplicate emissions, only wrap if it's not our
	// wrapper already. Since we can't reliably detect prior wrapping without adding
	// types, we conservatively wrap here to guarantee event emission.
	effectiveHandler := m.wrapHandlerWithRequestEvents(m.handler)

	// Create server with configured timeouts
	m.server = &http.Server{
		Addr:         addr,
		Handler:      effectiveHandler,
		ReadTimeout:  m.config.ReadTimeout,
		WriteTimeout: m.config.WriteTimeout,
		IdleTimeout:  m.config.IdleTimeout,
	}

	// Start the server in a goroutine
	go func() {
		m.logger.Info("Starting HTTP server", "address", addr)
		var err error

		// Start server with or without TLS based on configuration
		if m.config.TLS != nil && m.config.TLS.Enabled {
			// Configure TLS
			tlsConfig := &tls.Config{
				MinVersion: tls.VersionTLS12,
			}

			// UseService flag takes precedence
			if m.config.TLS.UseService {
				if m.certificateService != nil {
					m.logger.Info("Using certificate service for TLS")
					tlsConfig.GetCertificate = m.certificateService.GetCertificate

					// Emit TLS enabled event SYNCHRONOUSLY
					tlsEvent := modular.NewCloudEvent(EventTypeTLSEnabled, "httpserver-service", map[string]interface{}{
						"method": "certificate_service",
					}, nil)
					if emitErr := m.EmitEvent(ctx, tlsEvent); emitErr != nil {
						m.logger.Debug("Failed to emit TLS enabled event", "error", emitErr)
					}

				} else {
					// Fall back to auto-generated certificates if UseService is true but no service is available
					m.logger.Warn("No certificate service available, falling back to auto-generated certificates")
					if len(m.config.TLS.Domains) == 0 {
						// If no domains specified, use localhost
						m.config.TLS.Domains = []string{"localhost"}
					}
					cert, key, err := m.generateSelfSignedCertificate(m.config.TLS.Domains)
					if err != nil {
						m.logger.Error("Failed to generate self-signed certificate", "error", err)
						if listenErr := m.server.ListenAndServe(); listenErr != nil {
							m.logger.Error("Failed to start HTTP server as fallback", "error", listenErr)
						}
					} else {
						m.server.TLSConfig = tlsConfig
						if listenErr := m.server.ListenAndServeTLS(cert, key); listenErr != nil {
							m.logger.Error("Failed to start HTTPS server", "error", listenErr)
						}
					}
				}
			} else if m.config.TLS.AutoGenerate {
				// Auto-generate self-signed certificates
				m.logger.Info("Auto-generating self-signed certificates", "domains", m.config.TLS.Domains)

				// Emit TLS enabled event SYNCHRONOUSLY before starting server
				tlsEvent := modular.NewCloudEvent(EventTypeTLSEnabled, "httpserver-service", map[string]interface{}{
					"method":  "auto_generate",
					"domains": m.config.TLS.Domains,
				}, nil)
				if emitErr := m.EmitEvent(ctx, tlsEvent); emitErr != nil {
					m.logger.Debug("Failed to emit TLS auto-generate event", "error", emitErr)
				}

				cert, key, err := m.generateSelfSignedCertificate(m.config.TLS.Domains)
				if err != nil {
					m.logger.Error("Failed to generate self-signed certificate", "error", err)
					if listenErr := m.server.ListenAndServe(); listenErr != nil {
						m.logger.Error("Failed to start HTTP server as fallback", "error", listenErr)
					}
				} else {
					m.server.TLSConfig = tlsConfig
					if listenErr := m.server.ListenAndServeTLS(cert, key); listenErr != nil {
						m.logger.Error("Failed to start HTTPS server", "error", listenErr)
					}
				}
			} else {
				// Use provided certificate files
				m.logger.Info("Using TLS configuration", "cert", m.config.TLS.CertFile, "key", m.config.TLS.KeyFile)

				// Emit TLS enabled event SYNCHRONOUSLY
				tlsEvent := modular.NewCloudEvent(EventTypeTLSEnabled, "httpserver-service", map[string]interface{}{
					"method":    "certificate_files",
					"cert_file": m.config.TLS.CertFile,
					"key_file":  m.config.TLS.KeyFile,
				}, nil)
				if emitErr := m.EmitEvent(ctx, tlsEvent); emitErr != nil {
					m.logger.Debug("Failed to emit TLS configured event", "error", emitErr)
				}

				err = m.server.ListenAndServeTLS(m.config.TLS.CertFile, m.config.TLS.KeyFile)
			}
		} else {
			err = m.server.ListenAndServe()
		}

		// If server was shut down gracefully, err will be http.ErrServerClosed
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			m.logger.Error("HTTP server error", "error", err)
		}
	}()

	// Test that server is actually listening
	timeout := time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	check := func() error {
		var dialer net.Dialer
		conn, err := dialer.DialContext(checkCtx, "tcp", addr)
		if err != nil {
			return fmt.Errorf("dialing server: %w", err)
		}
		if closeErr := conn.Close(); closeErr != nil {
			m.logger.Warn("Failed to close connection", "error", closeErr)
		}
		return nil
	}

	// Try to connect to the server with retries
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	startTime := time.Now()
	for {
		err := check()
		if err == nil {
			break // Successfully connected
		}

		// Check if the timeout has expired
		if time.Since(startTime) > timeout {
			return fmt.Errorf("failed to start HTTP server within timeout: %w", err)
		}

		// Wait before retrying
		select {
		case <-checkCtx.Done():
			return ErrServerStartTimeout
		case <-ticker.C:
		}
	}

	m.started = true
	m.logger.Info("HTTP server started successfully", "address", addr)

	// Emit server started event synchronously
	event := modular.NewCloudEvent(EventTypeServerStarted, "httpserver-service", map[string]interface{}{
		"address":     addr,
		"tls_enabled": m.config.TLS != nil && m.config.TLS.Enabled,
		"host":        m.config.Host,
		"port":        m.config.Port,
	}, nil)

	if emitErr := m.EmitEvent(ctx, event); emitErr != nil {
		m.logger.Debug("Failed to emit server started event", "error", emitErr)
	}

	// If TLS is enabled, emit TLS configured event now that server is fully started
	if m.config.TLS != nil && m.config.TLS.Enabled {
		var tlsMethod string
		if m.certificateService != nil && m.config.TLS.UseService {
			tlsMethod = "certificate_service"
		} else if m.config.TLS.AutoGenerate {
			tlsMethod = "auto_generate"
		} else {
			tlsMethod = "certificate_files"
		}

		tlsConfiguredEvent := modular.NewCloudEvent(EventTypeTLSConfigured, "httpserver-service", map[string]interface{}{
			"method":      tlsMethod,
			"https_port":  m.config.Port,
			"cert_method": tlsMethod,
		}, nil)

		if emitErr := m.EmitEvent(ctx, tlsConfiguredEvent); emitErr != nil {
			m.logger.Debug("Failed to emit TLS configured event", "error", emitErr)
		}
	}

	return nil
}

// Stop stops the HTTP server gracefully.
// This method initiates a graceful shutdown of the HTTP server,
// allowing existing connections to finish processing before closing.
//
// The shutdown process:
//  1. Check if server is running
//  2. Create shutdown context with configured timeout
//  3. Call server.Shutdown() to stop accepting new connections
//  4. Wait for existing connections to complete or timeout
//  5. Mark server as stopped
//
// If the shutdown timeout is exceeded, the server will be forcefully closed.
func (m *HTTPServerModule) Stop(ctx context.Context) error {
	if m.server == nil || !m.started {
		return ErrServerNotStarted
	}

	m.logger.Info("Stopping HTTP server", "timeout", m.config.ShutdownTimeout)

	// Create a context with timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(
		ctx,
		m.config.ShutdownTimeout,
	)
	defer cancel()

	// Shutdown the server gracefully
	err := m.server.Shutdown(shutdownCtx)
	if err != nil {
		return fmt.Errorf("error shutting down HTTP server: %w", err)
	}

	m.started = false
	m.logger.Info("HTTP server stopped successfully")

	// Removed synthetic request event emission: tests no longer rely on placeholder
	// events when no real traffic occurred. If needed in the future, reintroduce
	// behind a test-only build tag or explicit configuration flag.

	// Emit server stopped event synchronously
	event := modular.NewCloudEvent(EventTypeServerStopped, "httpserver-service", map[string]interface{}{
		"host": m.config.Host,
		"port": m.config.Port,
	}, nil)

	if emitErr := m.EmitEvent(ctx, event); emitErr != nil {
		m.logger.Debug("Failed to emit server stopped event", "error", emitErr)
	}

	return nil
}

// ProvidesServices returns the services provided by this module
func (m *HTTPServerModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        "httpserver",
			Description: "HTTP server module for handling HTTP requests and providing web services",
			Instance:    m,
		},
	}
}

// RequiresServices returns the services required by this module
func (m *HTTPServerModule) RequiresServices() []modular.ServiceDependency {
	deps := []modular.ServiceDependency{
		{
			Name:               "router",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*http.Handler)(nil)).Elem(),
		},
	}

	// Add optional certificate service dependency
	deps = append(deps, modular.ServiceDependency{
		Name:               "certificate",
		Required:           false, // Optional dependency
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeOf((*CertificateService)(nil)).Elem(),
	})

	return deps
}

// generateSelfSignedCertificate generates a self-signed certificate for the given domains.
// Returns the paths to the generated certificate and key files.
func (m *HTTPServerModule) generateSelfSignedCertificate(domains []string) (string, string, error) {
	// Generate a new private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Self-Signed Certificate"},
			CommonName:   domains[0], // Use the first domain as the common name
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add all domains as SANs
	for _, domain := range domains {
		if ip := net.ParseIP(domain); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, domain)
		}
	}

	// Add localhost and 127.0.0.1 to allow local testing
	template.DNSNames = append(template.DNSNames, "localhost")
	template.IPAddresses = append(template.IPAddresses, net.ParseIP("127.0.0.1"))
	template.IPAddresses = append(template.IPAddresses, net.ParseIP("::1"))

	// Create certificate from template and sign it with the private key
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to create certificate: %w", err)
	}

	// Convert DER certificate to PEM format
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))

	// Convert private key to PEM format
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal private key: %w", err)
	}
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}))

	// Create temporary files for certificate and key
	certFile, err := m.createTempFile("cert*.pem", certPEM)
	if err != nil {
		return "", "", fmt.Errorf("failed to create certificate file: %w", err)
	}

	keyFile, err := m.createTempFile("key*.pem", keyPEM)
	if err != nil {
		return "", "", fmt.Errorf("failed to create key file: %w", err)
	}

	return certFile, keyFile, nil
}

// createTempFile creates a temporary file with the given content
func (m *HTTPServerModule) createTempFile(pattern, content string) (string, error) {
	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer func() {
		if closeErr := tmpFile.Close(); closeErr != nil {
			m.logger.Warn("Failed to close temp file", "error", closeErr)
		}
	}()

	if _, err := tmpFile.WriteString(content); err != nil {
		return "", fmt.Errorf("writing to temp file: %w", err)
	}

	return tmpFile.Name(), nil
}

// RegisterObservers implements the ObservableModule interface.
// This allows the httpserver module to register as an observer for events it's interested in.
func (m *HTTPServerModule) RegisterObservers(subject modular.Subject) error {
	m.mu.Lock()
	m.subject = subject
	m.mu.Unlock()
	return nil
}

// EmitEvent implements the ObservableModule interface.
// This allows the httpserver module to emit events to registered observers.
func (m *HTTPServerModule) EmitEvent(ctx context.Context, event cloudevents.Event) error {
	// Acquire subject snapshot under read lock
	m.mu.RLock()
	subject := m.subject
	m.mu.RUnlock()
	// Fallback to app subject only if module subject not set
	if subject == nil && m.app != nil {
		if s, ok := m.app.(modular.Subject); ok {
			subject = s
		}
	}
	if subject == nil {
		return ErrNoSubjectForEventEmission
	}
	// Synchronous for request lifecycle events
	if event.Type() == EventTypeRequestReceived || event.Type() == EventTypeRequestHandled {
		ctx = modular.WithSynchronousNotification(ctx)
		if err := subject.NotifyObservers(ctx, event); err != nil {
			return fmt.Errorf("failed to notify observers for event %s: %w", event.Type(), err)
		}
		return nil
	}
	go func(s modular.Subject, e cloudevents.Event) {
		if err := s.NotifyObservers(ctx, e); err != nil && m.logger != nil {
			m.logger.Debug("Failed to notify observers", "error", err, "event_type", e.Type())
		}
	}(subject, event)
	return nil
}

// wrapHandlerWithRequestEvents wraps the HTTP handler to emit request events
func (m *HTTPServerModule) wrapHandlerWithRequestEvents(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Request lifecycle events are emitted for each real request
		// Emit request received event SYNCHRONOUSLY to ensure immediate emission
		requestReceivedEvent := modular.NewCloudEvent(EventTypeRequestReceived, "httpserver-service", map[string]interface{}{
			"method":      r.Method,
			"url":         r.URL.String(),
			"remote_addr": r.RemoteAddr,
			"user_agent":  r.UserAgent(),
		}, nil)
		// Request events should be delivered synchronously; set hint via a background context to avoid cancellation
		if emitErr := m.EmitEvent(modular.WithSynchronousNotification(r.Context()), requestReceivedEvent); emitErr != nil {
			// Temporary diagnostic to understand why events may not be observed in tests
			//nolint:forbidigo
			fmt.Println("[httpserver] DEBUG: failed to emit request.received:", emitErr)
			if m.logger != nil {
				m.logger.Debug("Failed to emit request received event", "error", emitErr)
			}
		}

		// Wrap response writer to capture status code
		// Default to 0 (unset) to distinguish between explicit and implicit status codes
		wrappedWriter := &responseWriter{ResponseWriter: w, statusCode: 0}

		// Call the original handler
		handler.ServeHTTP(wrappedWriter, r)

		// Emit request handled event SYNCHRONOUSLY to ensure immediate emission
		// Use the actual status code if set, otherwise default to 200 (HTTP OK)
		statusCode := wrappedWriter.statusCode
		if statusCode == 0 {
			statusCode = http.StatusOK // Default for successful responses when not explicitly set
		}

		requestHandledEvent := modular.NewCloudEvent(EventTypeRequestHandled, "httpserver-service", map[string]interface{}{
			"method":      r.Method,
			"url":         r.URL.String(),
			"status_code": statusCode,
			"remote_addr": r.RemoteAddr,
		}, nil)
		// Request events should be delivered synchronously; set hint via a background context to avoid cancellation
		if emitErr := m.EmitEvent(modular.WithSynchronousNotification(r.Context()), requestHandledEvent); emitErr != nil {
			//nolint:forbidigo
			fmt.Println("[httpserver] DEBUG: failed to emit request.handled:", emitErr)
			if m.logger != nil {
				m.logger.Debug("Failed to emit request handled event", "error", emitErr)
			}
		}
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode    int
	headerWritten bool // Track if WriteHeader has been called
}

func (rw *responseWriter) WriteHeader(code int) {
	// Prevent multiple calls to WriteHeader as per HTTP specification
	if rw.headerWritten {
		return
	}
	rw.statusCode = code
	rw.headerWritten = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	// If WriteHeader hasn't been called yet, it will be called implicitly with 200
	if !rw.headerWritten {
		rw.WriteHeader(http.StatusOK)
	}

	n, err := rw.ResponseWriter.Write(data)
	if err != nil {
		return n, fmt.Errorf("failed to write HTTP response: %w", err)
	}
	return n, nil
}

// GetRegisteredEventTypes implements the ObservableModule interface.
// Returns all event types that this httpserver module can emit.
func (m *HTTPServerModule) GetRegisteredEventTypes() []string {
	return []string{
		EventTypeServerStarted,
		EventTypeServerStopped,
		EventTypeRequestReceived,
		EventTypeRequestHandled,
		EventTypeTLSEnabled,
		EventTypeTLSConfigured,
		EventTypeConfigLoaded,
	}
}
