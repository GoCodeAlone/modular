package modular

import (
	"context"
	"sync/atomic"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// observer stub
// Using FunctionalObserver to capture notifications.

type simpleModule struct{}

func (simpleModule) Name() string           { return "m1" }
func (simpleModule) Init(Application) error { return nil }

// Reuse dummyHealthProvider defined in application_options_additional_test.go

func TestBaseApplicationDecoratorForwarding(t *testing.T) {
	base := NewObservableApplication(NewStdConfigProvider(&struct{}{}), NewTestLogger())
	dec := NewBaseApplicationDecorator(base)

	// Basic forwards & getters
	if dec.GetInnerApplication() == nil {
		t.Fatalf("inner nil")
	}
	if dec.ConfigProvider() == nil {
		t.Fatalf("config provider nil")
	}
	if dec.SvcRegistry() == nil {
		t.Fatalf("svc registry nil")
	}
	if err := dec.RegisterService("svc", 123); err != nil {
		t.Fatalf("register service: %v", err)
	}
	var out int
	if err := dec.GetService("svc", &out); err != nil || out != 123 {
		t.Fatalf("get service: %v %d", err, out)
	}

	// Config sections forwarding (empty map ok)
	if dec.ConfigSections() == nil {
		t.Fatalf("config sections nil")
	}
	dec.RegisterConfigSection("sec1", NewStdConfigProvider(&struct {
		A int `yaml:"a"`
	}{}))
	if _, err := dec.GetConfigSection("sec1"); err != nil {
		t.Fatalf("get config section: %v", err)
	}

	// Logger forwarding
	oldLogger := dec.Logger()
	newLogger := NewTestLogger()
	dec.SetLogger(newLogger)
	if dec.Logger() != newLogger {
		t.Fatalf("logger not updated")
	}
	dec.SetVerboseConfig(true)
	if !dec.IsVerboseConfig() {
		t.Fatalf("verbose flag not set")
	}
	dec.SetLogger(oldLogger) // restore

	// ServiceIntrospector (may be nil); just call
	_ = dec.ServiceIntrospector()

	// RegisterModule forwarding (minimal module)
	dec.RegisterModule(simpleModule{})

	// Run forwarding: use a goroutine to send signal after short delay not feasible here; just invoke Start/Stop directly to exercise underlying
	if err := dec.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := dec.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := dec.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Observer related forwards
	var received int64
	obs := NewFunctionalObserver("test-observer", func(ctx context.Context, event cloudevents.Event) error {
		atomic.AddInt64(&received, 1)
		return nil
	})
	if err := dec.RegisterObserver(obs); err != nil {
		t.Fatalf("register observer: %v", err)
	}
	if len(dec.GetObservers()) == 0 {
		t.Fatalf("expected observers")
	}
	evt := cloudevents.NewEvent()
	evt.SetID("evt-1")
	evt.SetType("test.event")
	evt.SetSource("unit")
	// Use synchronous notification so the observer increment happens before assertion
	if err := dec.NotifyObservers(WithSynchronousNotification(context.Background()), evt); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if atomic.LoadInt64(&received) == 0 {
		t.Fatalf("observer not notified")
	}
	if err := dec.UnregisterObserver(obs); err != nil {
		t.Fatalf("unregister: %v", err)
	}

	// RequestReload forwarding (no-op acceptable)
	if err := dec.RequestReload("section1"); err == nil {
		t.Fatalf("expected reload error (no dynamic reload)")
	}

	// Health forwarding: register dummy provider then call Health aggregator
	dummyProv := &dummyHealthProvider{id: "dummy"}
	if err := dec.RegisterHealthProvider("dummy", dummyProv, false); err != nil {
		t.Fatalf("register health provider: %v", err)
	}
	if agg, err := dec.Health(); err == nil && agg == nil {
		t.Fatalf("expected aggregator when no error")
	}

	// Tenant methods (base not tenant aware); just ensure no panic
	_, _ = dec.GetTenantService()
	_, _ = dec.WithTenant("t1")
	_, _ = dec.GetTenantConfig("t1", "sec")
	_ = dec.GetTenantGuard()

	// Health aggregator forward (may return error if not configured; acceptable either way)
	_, _ = dec.Health()
}
