package httpserver

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newTestModule returns a minimally configured HTTPServerModule for unit tests.
func newTestModule(t *testing.T) *HTTPServerModule {
	t.Helper()
	logger := &MockLogger{}
	// Register logger mocks for varying arg counts (msg + 0, 2, or 4 keyval args).
	for _, method := range []string{"Info", "Debug", "Warn", "Error"} {
		logger.On(method, mock.Anything).Maybe()
		logger.On(method, mock.Anything, mock.Anything).Maybe()
		logger.On(method, mock.Anything, mock.Anything, mock.Anything).Maybe()
		logger.On(method, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
		logger.On(method, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	}

	return &HTTPServerModule{
		config: &HTTPServerConfig{
			Host:            "127.0.0.1",
			Port:            9999,
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    15 * time.Second,
			IdleTimeout:     60 * time.Second,
			ShutdownTimeout: 5 * time.Second,
		},
		logger: logger,
	}
}

// ---------------------------------------------------------------------------
// Drainable
// ---------------------------------------------------------------------------

func TestHTTPServerModule_Drainable(t *testing.T) {
	// Verify interface compliance at compile time.
	var _ modular.Drainable = (*HTTPServerModule)(nil)

	m := newTestModule(t)

	// Before PreStop, draining should be false.
	assert.False(t, m.draining, "draining flag should be false before PreStop")

	err := m.PreStop(context.Background())
	require.NoError(t, err)

	// After PreStop, draining should be true.
	assert.True(t, m.draining, "draining flag should be true after PreStop")
}

// ---------------------------------------------------------------------------
// Reloadable
// ---------------------------------------------------------------------------

func TestHTTPServerModule_Reloadable(t *testing.T) {
	var _ modular.Reloadable = (*HTTPServerModule)(nil)

	t.Run("CanReload false when not started", func(t *testing.T) {
		m := newTestModule(t)
		assert.False(t, m.CanReload())
	})

	t.Run("CanReload true when started", func(t *testing.T) {
		m := newTestModule(t)
		m.started = true
		assert.True(t, m.CanReload())
	})

	t.Run("ReloadTimeout is 5 seconds", func(t *testing.T) {
		m := newTestModule(t)
		assert.Equal(t, 5*time.Second, m.ReloadTimeout())
	})

	t.Run("Reload updates config timeouts", func(t *testing.T) {
		m := newTestModule(t)
		m.started = true
		m.server = &http.Server{
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		}

		changes := []modular.ConfigChange{
			{FieldPath: "ReadTimeout", NewValue: "30s"},
			{FieldPath: "WriteTimeout", NewValue: "25s"},
			{FieldPath: "httpserver.IdleTimeout", NewValue: "120s"},
		}

		err := m.Reload(context.Background(), changes)
		require.NoError(t, err)

		// Config is updated; server fields are not mutated to avoid data races
		// on a running http.Server (new values take effect on restart).
		assert.Equal(t, 30*time.Second, m.config.ReadTimeout)
		assert.Equal(t, 25*time.Second, m.config.WriteTimeout)
		assert.Equal(t, 120*time.Second, m.config.IdleTimeout)
	})

	t.Run("Reload rejects invalid duration", func(t *testing.T) {
		m := newTestModule(t)
		m.started = true
		m.server = &http.Server{}

		changes := []modular.ConfigChange{
			{FieldPath: "ReadTimeout", NewValue: "not-a-duration"},
		}

		err := m.Reload(context.Background(), changes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid ReadTimeout")
	})

	t.Run("Reload fails when server not started", func(t *testing.T) {
		m := newTestModule(t)

		err := m.Reload(context.Background(), []modular.ConfigChange{
			{FieldPath: "ReadTimeout", NewValue: "10s"},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "server not started")
	})

	t.Run("Reload ignores unknown fields", func(t *testing.T) {
		m := newTestModule(t)
		m.started = true
		m.server = &http.Server{}

		changes := []modular.ConfigChange{
			{FieldPath: "UnknownField", NewValue: "whatever"},
		}

		err := m.Reload(context.Background(), changes)
		require.NoError(t, err)
	})
}

// ---------------------------------------------------------------------------
// MetricsProvider
// ---------------------------------------------------------------------------

func TestHTTPServerModule_MetricsProvider(t *testing.T) {
	var _ modular.MetricsProvider = (*HTTPServerModule)(nil)

	t.Run("metrics when not started", func(t *testing.T) {
		m := newTestModule(t)

		metrics := m.CollectMetrics(context.Background())
		assert.Equal(t, ModuleName, metrics.Name)
		assert.Equal(t, 0.0, metrics.Values["started"])
		assert.Equal(t, float64(9999), metrics.Values["port"])
	})

	t.Run("metrics when started", func(t *testing.T) {
		m := newTestModule(t)
		m.started = true

		metrics := m.CollectMetrics(context.Background())
		assert.Equal(t, ModuleName, metrics.Name)
		assert.Equal(t, 1.0, metrics.Values["started"])
		assert.Equal(t, float64(9999), metrics.Values["port"])
	})
}
