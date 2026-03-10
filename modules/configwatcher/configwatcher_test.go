package configwatcher

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestConfigWatcher_DetectsFileChange(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("key: value1"), 0644); err != nil {
		t.Fatal(err)
	}

	var changeCount atomic.Int32
	w := New(
		WithPaths(cfgFile),
		WithDebounce(50*time.Millisecond),
		WithOnChange(func(paths []string) { changeCount.Add(1) }),
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
		if changeCount.Load() > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if changeCount.Load() == 0 {
		t.Error("expected at least one change notification")
	}
}

func TestConfigWatcher_Debounces(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgFile, []byte("v1"), 0644)

	var changeCount atomic.Int32
	w := New(
		WithPaths(cfgFile),
		WithDebounce(200*time.Millisecond),
		WithOnChange(func(paths []string) { changeCount.Add(1) }),
	)
	if err := w.startWatching(); err != nil {
		t.Fatalf("startWatching: %v", err)
	}
	defer w.stopWatching()

	time.Sleep(100 * time.Millisecond)
	for i := 0; i < 5; i++ {
		os.WriteFile(cfgFile, []byte("v"+string(rune('2'+i))), 0644)
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)

	if changeCount.Load() > 2 {
		t.Errorf("expected debounced to ~1-2 calls, got %d", changeCount.Load())
	}
}
