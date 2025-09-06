package modular

import (
	"context"
	"testing"
)

// noopLogger provides a minimal Logger implementation for tests in this package.
type noopLogger struct{}

func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Debug(string, ...any) {}

// minimalConfig used for simple config provider tests
type minimalConfig struct{ Value string }

func TestBaseApplicationDecoratorForwarding_New(t *testing.T) { // _New to avoid name clash if similar test exists
	cfg := &minimalConfig{Value: "ok"}
	cp := NewStdConfigProvider(cfg)
	logger := &noopLogger{}
	app := NewStdApplication(cp, logger)

	dec := NewBaseApplicationDecorator(app)

	if dec.ConfigProvider() != cp {
		t.Fatalf("expected forwarded ConfigProvider")
	}
	// register & retrieve config section forwarding
	otherCfg := &minimalConfig{Value: "section"}
	otherCP := NewStdConfigProvider(otherCfg)
	dec.RegisterConfigSection("other", otherCP)
	got, err := dec.GetConfigSection("other")
	if err != nil || got != otherCP {
		t.Fatalf("expected forwarded config section, err=%v", err)
	}
	// service registration / retrieval forwarding
	type svcType struct{ X int }
	svc := &svcType{X: 7}
	if err := dec.RegisterService("svc", svc); err != nil {
		t.Fatalf("register service: %v", err)
	}
	var fetched *svcType
	if err := dec.GetService("svc", &fetched); err != nil || fetched.X != 7 {
		t.Fatalf("get service failed: %v", err)
	}

	// verbose config flag forwarding
	dec.SetVerboseConfig(true)
	if !dec.IsVerboseConfig() {
		t.Fatalf("expected verbose config enabled")
	}

	// Methods that just forward and return nil should still be invoked to cover lines
	if err := dec.Init(); err != nil { // empty app
		t.Fatalf("Init forwarding failed: %v", err)
	}
	if err := dec.Start(); err != nil { // no modules
		t.Fatalf("Start forwarding failed: %v", err)
	}
	if err := dec.Stop(); err != nil { // no modules
		t.Fatalf("Stop forwarding failed: %v", err)
	}

	// Observer / tenant aware branches when inner does not implement those interfaces
	obsErr := dec.RegisterObserver(nil)
	if obsErr == nil { // nil observer & not subject => should error with ErrServiceNotFound
		t.Fatalf("expected error for RegisterObserver when inner not Subject")
	}
	if err := dec.NotifyObservers(context.Background(), NewCloudEvent("x", "y", nil, nil)); err == nil {
		t.Fatalf("expected error for NotifyObservers when inner not Subject")
	}
}
