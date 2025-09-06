package chimux

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
    "context"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/assert"
    cloudevents "github.com/cloudevents/sdk-go/v2"
)

// testEventCollector is a simple observer to capture emitted events
type testEventCollector struct { events []string }
func (c *testEventCollector) OnEvent(ctx context.Context, e cloudevents.Event) error { c.events = append(c.events, e.Type()); return nil }
func (c *testEventCollector) ObserverID() string { return "collector" }

func TestDisableRoute_Basic(t *testing.T) {
    module := NewChiMuxModule().(*ChiMuxModule)
    mockApp := NewMockApplication()

    // Register config & observers
    err := module.RegisterConfig(mockApp)
    require.NoError(t, err)
    collector := &testEventCollector{}
    require.NoError(t, mockApp.RegisterObserver(collector))
    require.NoError(t, module.RegisterObservers(mockApp))
    require.NoError(t, module.Init(mockApp))

    // Register route
    module.Get("/disable-me", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK); _, _ = w.Write([]byte("alive")) })

    // Sanity request
    w1 := httptest.NewRecorder()
    req1 := httptest.NewRequest("GET", "/disable-me", nil)
    module.router.ServeHTTP(w1, req1)
    require.Equal(t, http.StatusOK, w1.Code)

    // Disable
    err = module.DisableRoute("GET", "/disable-me")
    require.NoError(t, err)
    assert.True(t, module.IsRouteDisabled("GET", "/disable-me"))

    // Request now 404
    w2 := httptest.NewRecorder()
    req2 := httptest.NewRequest("GET", "/disable-me", nil)
    module.router.ServeHTTP(w2, req2)
    assert.Equal(t, http.StatusNotFound, w2.Code)

    // Allow async event emission if any
    time.Sleep(10 * time.Millisecond)

    // Ensure route removed event present
    found := false
    for _, et := range collector.events { if et == EventTypeRouteRemoved { found = true; break } }
    assert.True(t, found, "expected route removed event")
}

func TestDisableRoute_NotFound(t *testing.T) {
    module := NewChiMuxModule().(*ChiMuxModule)
    mockApp := NewMockApplication()
    require.NoError(t, module.RegisterConfig(mockApp))
    require.NoError(t, module.RegisterObservers(mockApp))
    require.NoError(t, module.Init(mockApp))
    err := module.DisableRoute("GET", "/missing")
    require.Error(t, err)
}
