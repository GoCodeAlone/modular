package httpserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Ensure HTTPServerModule implements the Reloadable interface
var _ modular.Reloadable = (*HTTPServerModule)(nil)

// Reload applies configuration changes to the HTTP server module
// This method implements the modular.Reloadable interface for dynamic configuration updates
func (m *HTTPServerModule) Reload(ctx context.Context, changes []modular.ConfigChange) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.CanReload() {
		return fmt.Errorf("httpserver module is not in a reloadable state")
	}

	// Track changes by field for efficient processing
	changeMap := make(map[string]modular.ConfigChange)
	for _, change := range changes {
		if change.Section == "httpserver" {
			changeMap[change.FieldPath] = change
		}
	}

	if len(changeMap) == 0 {
		return nil // No changes for this module
	}

	// Validate all changes before applying any
	if err := m.validateReloadChanges(changeMap); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Apply changes based on field type
	if err := m.applyReloadChanges(ctx, changeMap); err != nil {
		return fmt.Errorf("failed to apply configuration changes: %w", err)
	}

	// Emit configuration reload event
	m.emitConfigReloadedEvent(changes)

	return nil
}

// CanReload returns true if the HTTP server supports dynamic reloading
func (m *HTTPServerModule) CanReload() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Can reload if the module is started and has a valid configuration
	return m.started && m.config != nil && m.server != nil
}

// ReloadTimeout returns the maximum time needed to complete a reload
func (m *HTTPServerModule) ReloadTimeout() time.Duration {
	return 10 * time.Second // HTTP server reloads should complete within 10 seconds
}

// validateReloadChanges validates that all proposed changes are valid and safe to apply
func (m *HTTPServerModule) validateReloadChanges(changes map[string]modular.ConfigChange) error {
	for fieldPath, change := range changes {
		switch fieldPath {
		case "httpserver.read_timeout":
			if duration, ok := change.NewValue.(time.Duration); ok {
				if duration < 0 {
					return fmt.Errorf("read_timeout cannot be negative: %v", duration)
				}
			} else {
				return fmt.Errorf("read_timeout must be a time.Duration, got %T", change.NewValue)
			}

		case "httpserver.write_timeout":
			if duration, ok := change.NewValue.(time.Duration); ok {
				if duration < 0 {
					return fmt.Errorf("write_timeout cannot be negative: %v", duration)
				}
			} else {
				return fmt.Errorf("write_timeout must be a time.Duration, got %T", change.NewValue)
			}

		case "httpserver.idle_timeout":
			if duration, ok := change.NewValue.(time.Duration); ok {
				if duration < 0 {
					return fmt.Errorf("idle_timeout cannot be negative: %v", duration)
				}
			} else {
				return fmt.Errorf("idle_timeout must be a time.Duration, got %T", change.NewValue)
			}

		case "httpserver.tls.enabled":
			if _, ok := change.NewValue.(bool); !ok {
				return fmt.Errorf("tls.enabled must be a boolean, got %T", change.NewValue)
			}

		case "httpserver.tls.cert_file", "httpserver.tls.key_file":
			if _, ok := change.NewValue.(string); !ok {
				return fmt.Errorf("%s must be a string, got %T", fieldPath, change.NewValue)
			}

		case "httpserver.address", "httpserver.port":
			// These require server restart and cannot be reloaded dynamically
			return fmt.Errorf("field %s requires server restart and cannot be reloaded dynamically", fieldPath)

		default:
			// Allow unknown fields to be processed - they might be added in the future
			if m.logger != nil {
				m.logger.Warn("Unknown httpserver configuration field in reload", "field_path", fieldPath)
			}
		}
	}

	return nil
}

// applyReloadChanges applies the validated configuration changes
func (m *HTTPServerModule) applyReloadChanges(ctx context.Context, changes map[string]modular.ConfigChange) error {
	// Track whether we need to update server configuration
	needsServerUpdate := false

	// Apply timeout changes
	if change, exists := changes["httpserver.read_timeout"]; exists {
		if duration, ok := change.NewValue.(time.Duration); ok {
			m.config.ReadTimeout = duration
			needsServerUpdate = true
		}
	}

	if change, exists := changes["httpserver.write_timeout"]; exists {
		if duration, ok := change.NewValue.(time.Duration); ok {
			m.config.WriteTimeout = duration
			needsServerUpdate = true
		}
	}

	if change, exists := changes["httpserver.idle_timeout"]; exists {
		if duration, ok := change.NewValue.(time.Duration); ok {
			m.config.IdleTimeout = duration
			needsServerUpdate = true
		}
	}

	// Apply TLS configuration changes
	if change, exists := changes["httpserver.tls.enabled"]; exists {
		if enabled, ok := change.NewValue.(bool); ok {
			m.config.TLS.Enabled = enabled
			needsServerUpdate = true
		}
	}

	if change, exists := changes["httpserver.tls.cert_file"]; exists {
		if certFile, ok := change.NewValue.(string); ok {
			m.config.TLS.CertFile = certFile
			needsServerUpdate = true
		}
	}

	if change, exists := changes["httpserver.tls.key_file"]; exists {
		if keyFile, ok := change.NewValue.(string); ok {
			m.config.TLS.KeyFile = keyFile
			needsServerUpdate = true
		}
	}

	// Update server configuration if needed
	if needsServerUpdate {
		if err := m.updateServerConfiguration(ctx); err != nil {
			return fmt.Errorf("failed to update server configuration: %w", err)
		}
	}

	return nil
}

// updateServerConfiguration applies the new configuration to the running server
func (m *HTTPServerModule) updateServerConfiguration(ctx context.Context) error {
	if m.server == nil {
		return fmt.Errorf("server is not initialized")
	}

	// Update timeouts
	m.server.ReadTimeout = m.config.ReadTimeout
	m.server.WriteTimeout = m.config.WriteTimeout
	m.server.IdleTimeout = m.config.IdleTimeout

	// Update TLS configuration if needed
	if m.config.TLS.Enabled && (m.config.TLS.CertFile != "" && m.config.TLS.KeyFile != "") {
		if err := m.reloadTLSConfiguration(ctx); err != nil {
			return fmt.Errorf("failed to reload TLS configuration: %w", err)
		}
	}

	if m.logger != nil {
		m.logger.Info("HTTP server configuration reloaded successfully",
			"read_timeout", m.config.ReadTimeout,
			"write_timeout", m.config.WriteTimeout,
			"idle_timeout", m.config.IdleTimeout,
			"tls_enabled", m.config.TLS.Enabled,
		)
	}

	return nil
}

// reloadTLSConfiguration reloads TLS certificates and configuration
func (m *HTTPServerModule) reloadTLSConfiguration(ctx context.Context) error {
	if !m.config.TLS.Enabled || m.config.TLS.CertFile == "" || m.config.TLS.KeyFile == "" {
		return nil
	}

	// Load new TLS certificate
	cert, err := m.loadTLSCertificate(m.config.TLS.CertFile, m.config.TLS.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	// Update server TLS configuration
	if m.server.TLSConfig == nil {
		m.server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	m.server.TLSConfig.Certificates = []tls.Certificate{cert}

	if m.logger != nil {
		m.logger.Info("TLS configuration reloaded successfully",
			"cert_file", m.config.TLS.CertFile,
			"key_file", m.config.TLS.KeyFile,
		)
	}

	return nil
}

// loadTLSCertificate loads a TLS certificate from the specified files
func (m *HTTPServerModule) loadTLSCertificate(certFile, keyFile string) (tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load x509 key pair: %w", err)
	}
	return cert, nil
}

// emitConfigReloadedEvent emits an event indicating successful configuration reload
func (m *HTTPServerModule) emitConfigReloadedEvent(changes []modular.ConfigChange) {
	if m.subject == nil {
		return
	}

	// Create a CloudEvents event
	event := cloudevents.NewEvent()
	event.SetType("httpserver.config.reloaded")
	event.SetSource("modular.httpserver")
	event.SetSubject(ModuleName)
	event.SetTime(time.Now())
	event.SetID(fmt.Sprintf("config-reload-%d", time.Now().UnixNano()))

	eventData := HTTPServerConfigReloadedEvent{
		ModuleName: ModuleName,
		Timestamp:  time.Now(),
		Changes:    changes,
	}

	if err := event.SetData(cloudevents.ApplicationJSON, eventData); err != nil {
		if m.logger != nil {
			m.logger.Error("Failed to set event data", "error", err)
		}
		return
	}

	ctx := context.Background()
	if err := m.subject.NotifyObservers(ctx, event); err != nil {
		if m.logger != nil {
			m.logger.Error("Failed to notify observers of config reload", "error", err)
		}
	}
}

// HTTPServerConfigReloadedEvent represents a configuration reload event
type HTTPServerConfigReloadedEvent struct {
	ModuleName string                 `json:"module_name"`
	Timestamp  time.Time              `json:"timestamp"`
	Changes    []modular.ConfigChange `json:"changes"`
}
