package configwatcher

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
)

// mockReloadableApp is a minimal modular.ReloadableApp used in tests.
type mockReloadableApp struct {
	reloadCount atomic.Int32
	lastTrigger atomic.Int32
}

func (m *mockReloadableApp) RequestReload(_ context.Context, trigger modular.ReloadTrigger, _ modular.ConfigDiff) error {
	m.reloadCount.Add(1)
	m.lastTrigger.Store(int32(trigger))
	return nil
}

func TestWithAutoReload_TriggersReloadOnFileChange(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("key: value1"), 0644); err != nil {
		t.Fatal(err)
	}

	app := &mockReloadableApp{}

	w := New(
		WithPaths(cfgFile),
		WithDebounce(50*time.Millisecond),
		WithAutoReload(app),
	)

	if err := w.startWatching(); err != nil {
		t.Fatalf("startWatching: %v", err)
	}
	defer w.stopWatching()

	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(cfgFile, []byte("key: value2"), 0644); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if app.reloadCount.Load() > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if app.reloadCount.Load() == 0 {
		t.Error("expected RequestReload to be called at least once")
	}
	if got := modular.ReloadTrigger(app.lastTrigger.Load()); got != modular.ReloadFileChange {
		t.Errorf("expected trigger %v, got %v", modular.ReloadFileChange, got)
	}
}

func TestConnectAutoReload_TriggersReloadOnFileChange(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("key: original"), 0644); err != nil {
		t.Fatal(err)
	}

	app := &mockReloadableApp{}

	w := New(
		WithPaths(cfgFile),
		WithDebounce(50*time.Millisecond),
	)
	// Wire after construction via ConnectAutoReload.
	ConnectAutoReload(w, app)

	if err := w.startWatching(); err != nil {
		t.Fatalf("startWatching: %v", err)
	}
	defer w.stopWatching()

	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(cfgFile, []byte("key: updated"), 0644); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if app.reloadCount.Load() > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if app.reloadCount.Load() == 0 {
		t.Error("expected RequestReload to be called at least once after ConnectAutoReload")
	}
	if got := modular.ReloadTrigger(app.lastTrigger.Load()); got != modular.ReloadFileChange {
		t.Errorf("expected trigger %v, got %v", modular.ReloadFileChange, got)
	}
}
