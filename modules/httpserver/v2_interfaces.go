package httpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Compile-time interface assertions for v2 enhancement interfaces.
var (
	_ modular.Drainable       = (*HTTPServerModule)(nil)
	_ modular.Reloadable      = (*HTTPServerModule)(nil)
	_ modular.MetricsProvider = (*HTTPServerModule)(nil)
)

// PreStop signals that the server is entering the drain phase.
// The actual graceful shutdown (http.Server.Shutdown) happens in Stop().
// PreStop sets the draining flag so middleware or health checks can
// report the server as unhealthy during the drain window.
func (m *HTTPServerModule) PreStop(ctx context.Context) error {
	m.mu.Lock()
	m.draining = true
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Info("HTTP server entering drain phase")
	}
	return nil
}

// CanReload reports whether the module can currently accept a reload.
// Returns true only when the server has been started and is running.
func (m *HTTPServerModule) CanReload() bool {
	return m.started
}

// ReloadTimeout returns the maximum duration allowed for a reload operation.
func (m *HTTPServerModule) ReloadTimeout() time.Duration {
	return 5 * time.Second
}

// Reload applies configuration changes to the running HTTP server.
// Supported fields: ReadTimeout, WriteTimeout, IdleTimeout.
// Note: http.Server timeout fields are not safe for concurrent mutation on a
// running server, so only the config is updated here. The new values take
// effect if the server is restarted.
func (m *HTTPServerModule) Reload(_ context.Context, changes []modular.ConfigChange) error {
	if !m.started || m.server == nil {
		return ErrServerNotStarted
	}

	for _, change := range changes {
		field := change.FieldPath
		// Normalise: accept both dotted paths (e.g. "httpserver.ReadTimeout")
		// and bare field names.
		if idx := strings.LastIndex(field, "."); idx >= 0 {
			field = field[idx+1:]
		}
		field = strings.ToLower(field)

		switch field {
		case "readtimeout", "read_timeout":
			d, err := time.ParseDuration(change.NewValue)
			if err != nil {
				return fmt.Errorf("invalid ReadTimeout value %q: %w", change.NewValue, err)
			}
			m.config.ReadTimeout = d

		case "writetimeout", "write_timeout":
			d, err := time.ParseDuration(change.NewValue)
			if err != nil {
				return fmt.Errorf("invalid WriteTimeout value %q: %w", change.NewValue, err)
			}
			m.config.WriteTimeout = d

		case "idletimeout", "idle_timeout":
			d, err := time.ParseDuration(change.NewValue)
			if err != nil {
				return fmt.Errorf("invalid IdleTimeout value %q: %w", change.NewValue, err)
			}
			m.config.IdleTimeout = d
		}
	}

	return nil
}

// CollectMetrics returns operational metrics for the HTTP server module.
func (m *HTTPServerModule) CollectMetrics(ctx context.Context) modular.ModuleMetrics {
	started := 0.0
	if m.started {
		started = 1.0
	}

	port := 0.0
	if m.config != nil {
		port = float64(m.config.Port)
	}

	return modular.ModuleMetrics{
		Name: ModuleName,
		Values: map[string]float64{
			"started": started,
			"port":    port,
		},
	}
}
