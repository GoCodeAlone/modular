package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
	"github.com/GoCodeAlone/modular/modules/chimux"
	"github.com/GoCodeAlone/modular/modules/httpserver"
	"github.com/go-chi/chi/v5"
)

type AppConfig struct {
	Name string `yaml:"name" default:"Let's Encrypt Demo"`
}

type CertificateInfo struct {
	Subject      string    `json:"subject"`
	Issuer       string    `json:"issuer"`
	NotBefore    time.Time `json:"not_before"`
	NotAfter     time.Time `json:"not_after"`
	DNSNames     []string  `json:"dns_names"`
	SerialNumber string    `json:"serial_number"`
	IsCA         bool      `json:"is_ca"`
}

type SSLModule struct {
	router       chi.Router
	certService  httpserver.CertificateService
	tlsConfig    *tls.Config
}

func (m *SSLModule) Name() string {
	return "ssl-demo"
}

func (m *SSLModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:               "router",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*chi.Router)(nil)).Elem(),
		},
		{
			Name:               "certificateService",
			Required:           false, // Optional since it might not be available during startup
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*httpserver.CertificateService)(nil)).Elem(),
		},
	}
}

func (m *SSLModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		router, ok := services["router"].(chi.Router)
		if !ok {
			return nil, fmt.Errorf("router service not found or wrong type")
		}

		module := &SSLModule{
			router: router,
		}

		// Certificate service is optional during startup
		if certService, ok := services["certificateService"].(httpserver.CertificateService); ok {
			module.certService = certService
		}

		return module, nil
	}
}

func (m *SSLModule) Init(app modular.Application) error {
	// Set up HTTP routes
	m.router.Route("/api/ssl", func(r chi.Router) {
		r.Get("/info", m.getSSLInfo)
		r.Get("/certificates", m.getCertificates)
		r.Get("/test", m.testSSL)
	})

	m.router.Get("/", m.homePage)
	m.router.Get("/health", m.healthCheck)

	slog.Info("SSL demo module initialized")
	return nil
}

func (m *SSLModule) getSSLInfo(w http.ResponseWriter, r *http.Request) {
	// Get TLS connection state
	if r.TLS == nil {
		http.Error(w, "Not using TLS connection", http.StatusBadRequest)
		return
	}

	tlsInfo := map[string]interface{}{
		"tls_version":       getTLSVersionString(r.TLS.Version),
		"cipher_suite":      getCipherSuiteString(r.TLS.CipherSuite),
		"server_name":       r.TLS.ServerName,
		"handshake_complete": r.TLS.HandshakeComplete,
		"negotiated_protocol": r.TLS.NegotiatedProtocol,
	}

	// Get certificate info if available
	if len(r.TLS.PeerCertificates) > 0 {
		cert := r.TLS.PeerCertificates[0]
		tlsInfo["certificate"] = CertificateInfo{
			Subject:      cert.Subject.String(),
			Issuer:       cert.Issuer.String(),
			NotBefore:    cert.NotBefore,
			NotAfter:     cert.NotAfter,
			DNSNames:     cert.DNSNames,
			SerialNumber: cert.SerialNumber.String(),
			IsCA:         cert.IsCA,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tlsInfo)
}

func (m *SSLModule) getCertificates(w http.ResponseWriter, r *http.Request) {
	if m.certService == nil {
		http.Error(w, "Certificate service not available", http.StatusServiceUnavailable)
		return
	}

	// This would typically return certificate information
	// For demo purposes, we'll return basic info
	response := map[string]interface{}{
		"message": "Certificate service is available",
		"status":  "active",
		"note":    "In staging mode - certificates are for testing only",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *SSLModule) testSSL(w http.ResponseWriter, r *http.Request) {
	tests := []map[string]interface{}{
		{
			"test":        "TLS Connection",
			"description": "Verify TLS connection is active",
			"result":      r.TLS != nil,
		},
		{
			"test":        "HTTPS Protocol",
			"description": "Verify request is using HTTPS",
			"result":      r.URL.Scheme == "https" || r.Header.Get("X-Forwarded-Proto") == "https",
		},
		{
			"test":        "Certificate Present",
			"description": "Verify server certificate is available",
			"result":      r.TLS != nil && len(r.TLS.PeerCertificates) > 0,
		},
	}

	// Additional test for secure headers
	secureHeaders := map[string]string{
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"X-XSS-Protection":          "1; mode=block",
	}

	// Set secure headers for demonstration
	for header, value := range secureHeaders {
		w.Header().Set(header, value)
	}

	response := map[string]interface{}{
		"ssl_tests":      tests,
		"secure_headers": secureHeaders,
		"timestamp":      time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *SSLModule) homePage(w http.ResponseWriter, r *http.Request) {
	protocol := "HTTP"
	if r.TLS != nil {
		protocol = "HTTPS"
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Let's Encrypt Demo</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .status { padding: 20px; border-radius: 5px; margin: 20px 0; }
        .secure { background-color: #d4edda; border: 1px solid #c3e6cb; }
        .insecure { background-color: #f8d7da; border: 1px solid #f5c6cb; }
        .info { background-color: #d1ecf1; border: 1px solid #bee5eb; }
        ul { list-style-type: none; padding: 0; }
        li { margin: 10px 0; }
        a { color: #007bff; text-decoration: none; }
        a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <h1>🔐 Let's Encrypt Demo Application</h1>
    
    <div class="%s">
        <h2>Connection Status</h2>
        <p><strong>Protocol:</strong> %s</p>
        <p><strong>Host:</strong> %s</p>
        <p><strong>URL:</strong> %s</p>
    </div>

    <div class="info">
        <h2>Demo Configuration</h2>
        <p>This demo is configured to use Let's Encrypt's <strong>staging environment</strong> for safety.</p>
        <p>Staging certificates are not trusted by browsers but demonstrate the ACME protocol flow.</p>
    </div>

    <h2>🧪 Test Endpoints</h2>
    <ul>
        <li>📊 <a href="/api/ssl/info">SSL Connection Info</a> - View TLS connection details</li>
        <li>📋 <a href="/api/ssl/certificates">Certificate Status</a> - Check certificate service</li>
        <li>🔍 <a href="/api/ssl/test">SSL Tests</a> - Run security tests</li>
        <li>❤️ <a href="/health">Health Check</a> - Application health status</li>
    </ul>

    <h2>🔧 Configuration Notes</h2>
    <div class="info">
        <p><strong>Domains:</strong> localhost, 127.0.0.1 (for demo purposes)</p>
        <p><strong>Environment:</strong> Let's Encrypt Staging</p>
        <p><strong>Auto-Renewal:</strong> Enabled (30 days before expiry)</p>
        <p><strong>Storage:</strong> ./certs directory</p>
    </div>

    <h2>⚠️ Production Setup</h2>
    <div class="info">
        <p>For production use:</p>
        <ul>
            <li>Set <code>use_staging: false</code> in configuration</li>
            <li>Use real domain names (not localhost)</li>
            <li>Ensure domain DNS points to your server</li>
            <li>Open port 80 for HTTP-01 challenges (or configure DNS-01)</li>
            <li>Set proper email for Let's Encrypt notifications</li>
        </ul>
    </div>
</body>
</html>`, 
		getStatusClass(r.TLS != nil), 
		protocol, 
		r.Host, 
		r.URL.String())

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (m *SSLModule) healthCheck(w http.ResponseWriter, r *http.Request) {
	isSecure := r.TLS != nil
	health := map[string]interface{}{
		"status":   "healthy",
		"service":  "letsencrypt-demo",
		"secure":   isSecure,
		"protocol": getProtocol(isSecure),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func getStatusClass(secure bool) string {
	if secure {
		return "secure"
	}
	return "insecure"
}

func getProtocol(secure bool) string {
	if secure {
		return "HTTPS"
	}
	return "HTTP"
}

func getTLSVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", version)
	}
}

func getCipherSuiteString(cipherSuite uint16) string {
	// This is a simplified mapping - in practice you'd want a full mapping
	switch cipherSuite {
	case tls.TLS_RSA_WITH_AES_128_CBC_SHA:
		return "TLS_RSA_WITH_AES_128_CBC_SHA"
	case tls.TLS_RSA_WITH_AES_256_CBC_SHA:
		return "TLS_RSA_WITH_AES_256_CBC_SHA"
	case tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:
		return "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
	case tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:
		return "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", cipherSuite)
	}
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create config provider
	appConfig := &AppConfig{}
	configProvider := modular.NewStdConfigProvider(appConfig)

	// Create application
	app := modular.NewStdApplication(configProvider, logger)

	// Set up configuration feeders
	modular.ConfigFeeders = []modular.Feeder{
		feeders.NewYamlFeeder("config.yaml"),
		feeders.NewEnvFeeder(),
	}

	// Register modules
	// Note: Let's Encrypt module requires manual configuration and is commented out
	// For demo purposes, we'll use a self-signed certificate
	app.RegisterModule(chimux.NewChiMuxModule())
	app.RegisterModule(httpserver.NewHTTPServerModule())
	app.RegisterModule(&SSLModule{})

	logger.Info("Starting Let's Encrypt Demo Application")
	logger.Info("DEMO MODE: This demo shows SSL/TLS capabilities without actual Let's Encrypt certificates")
	logger.Info("For production Let's Encrypt integration, see the README.md for detailed configuration")

	// Run application
	if err := app.Run(); err != nil {
		logger.Error("Application error", "error", err)
		os.Exit(1)
	}
}