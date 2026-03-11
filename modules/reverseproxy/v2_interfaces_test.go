package reverseproxy

import (
	"context"
	"io"
	"log/slog"
	"net/http/httputil"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReverseProxyModule_CollectMetrics(t *testing.T) {
	t.Run("metrics enabled with data", func(t *testing.T) {
		module := NewModule()
		module.enableMetrics = true
		module.metrics = NewMetricsCollector()
		module.backendProxies = map[string]*httputil.ReverseProxy{
			"api1": {},
			"api2": {},
			"api3": {},
		}

		// Simulate some recorded requests
		module.metrics.RecordRequest("api1", time.Now().Add(-10*time.Millisecond), 200, nil)
		module.metrics.RecordRequest("api1", time.Now().Add(-5*time.Millisecond), 200, nil)
		module.metrics.RecordRequest("api2", time.Now().Add(-8*time.Millisecond), 500, assert.AnError)

		result := module.CollectMetrics(context.Background())

		assert.Equal(t, "reverseproxy", result.Name)
		require.NotNil(t, result.Values)
		assert.Equal(t, float64(3), result.Values["backend_count"])
		assert.Equal(t, float64(3), result.Values["total_requests"])
		assert.Equal(t, float64(1), result.Values["total_errors"])
	})

	t.Run("metrics disabled returns only backend_count", func(t *testing.T) {
		module := NewModule()
		module.enableMetrics = false
		module.backendProxies = map[string]*httputil.ReverseProxy{
			"api1": {},
		}

		result := module.CollectMetrics(context.Background())

		assert.Equal(t, "reverseproxy", result.Name)
		require.NotNil(t, result.Values)
		assert.Equal(t, float64(1), result.Values["backend_count"])
		_, hasRequests := result.Values["total_requests"]
		assert.False(t, hasRequests, "should not have total_requests when metrics disabled")
		_, hasErrors := result.Values["total_errors"]
		assert.False(t, hasErrors, "should not have total_errors when metrics disabled")
	})

	t.Run("no backends returns zero backend_count", func(t *testing.T) {
		module := NewModule()
		module.enableMetrics = false
		module.backendProxies = map[string]*httputil.ReverseProxy{}

		result := module.CollectMetrics(context.Background())

		assert.Equal(t, float64(0), result.Values["backend_count"])
	})

	t.Run("satisfies MetricsProvider interface", func(t *testing.T) {
		var _ modular.MetricsProvider = (*ReverseProxyModule)(nil)
	})
}

func TestReverseProxyModule_PreStop(t *testing.T) {
	t.Run("stops health checker", func(t *testing.T) {
		module := NewModule()
		// Create a minimal health checker that we can verify gets stopped
		hc := &HealthChecker{
			running:  true,
			stopChan: make(chan struct{}),
			logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		}
		module.healthChecker = hc

		err := module.PreStop(context.Background())

		require.NoError(t, err)
		assert.False(t, hc.running, "health checker should be stopped after PreStop")
	})

	t.Run("nil health checker does not panic", func(t *testing.T) {
		module := NewModule()
		module.healthChecker = nil

		err := module.PreStop(context.Background())

		require.NoError(t, err)
	})

	t.Run("satisfies Drainable interface", func(t *testing.T) {
		var _ modular.Drainable = (*ReverseProxyModule)(nil)
	})
}
