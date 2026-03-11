package modular

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type reloadableTestModule struct {
	name        string
	reloadCount atomic.Int32
}

func (m *reloadableTestModule) Name() string                 { return m.name }
func (m *reloadableTestModule) Init(app Application) error   { return nil }
func (m *reloadableTestModule) CanReload() bool              { return true }
func (m *reloadableTestModule) ReloadTimeout() time.Duration { return 5 * time.Second }
func (m *reloadableTestModule) Reload(ctx context.Context, changes []ConfigChange) error {
	m.reloadCount.Add(1)
	return nil
}

func TestWithDynamicReload_WiresOrchestrator(t *testing.T) {
	mod := &reloadableTestModule{name: "hot-mod"}
	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithModules(mod),
		WithDynamicReload(),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}
	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := app.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	stdApp := app.(*StdApplication)
	diff := ConfigDiff{
		Changed: map[string]FieldChange{
			"key": {OldValue: "old", NewValue: "new", FieldPath: "key", ChangeType: ChangeModified},
		},
		DiffID: "test-diff",
	}
	if err := stdApp.RequestReload(context.Background(), ReloadManual, diff); err != nil {
		t.Fatalf("RequestReload: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mod.reloadCount.Load() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if mod.reloadCount.Load() == 0 {
		t.Error("expected module to be reloaded")
	}

	if err := app.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestRequestReload_WithoutDynamicReload(t *testing.T) {
	app, err := NewApplication(WithLogger(nopLogger{}))
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}
	stdApp := app.(*StdApplication)
	err = stdApp.RequestReload(context.Background(), ReloadManual, ConfigDiff{})
	if err == nil {
		t.Error("expected error when dynamic reload not enabled")
	}
}
