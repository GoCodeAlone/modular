package reload

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// snapshotTestReloadable simulates a module whose config reads must be atomic during reload.
type snapshotTestReloadable struct {
	mu      sync.RWMutex
	current map[string]any
	// applied counter for verifying serialization
	applied int32
}

func newSnapshotReloadable(cfg map[string]any) *snapshotTestReloadable { return &snapshotTestReloadable{current: cfg} }

func (s *snapshotTestReloadable) Reload(ctx context.Context, changes []modular.ConfigChange) error {
	// Validate first (atomic semantics): gather new state then commit under lock.
	next := make(map[string]any, len(s.current))
	s.mu.RLock()
	for k, v := range s.current { next[k] = v }
	s.mu.RUnlock()
	for _, c := range changes {
		if c.NewValue == "fail" { return assert.AnError }
		// field paths simple: config.key
		parts := c.FieldPath
		// simplified: we expect single-level keys for test (e.g., log.level)
		next[parts] = c.NewValue
	}
	// Commit
	s.mu.Lock()
	s.current = next
	s.mu.Unlock()
	atomic.AddInt32(&s.applied, 1)
	return nil
}
func (s *snapshotTestReloadable) CanReload() bool          { return true }
func (s *snapshotTestReloadable) ReloadTimeout() time.Duration { return 2 * time.Second }
func (s *snapshotTestReloadable) Read(key string) any {
	s.mu.RLock(); defer s.mu.RUnlock(); return s.current[key]
}

// buildDiff helper for tests.
func buildDiff(oldCfg, newCfg map[string]any) *modular.ConfigDiff {
	d, _ := modular.GenerateConfigDiff(oldCfg, newCfg); return d
}

func TestReloadRaceSafety(t *testing.T) {
	manager := NewReloadManager([]string{"log.level"})
	base := map[string]any{"log.level": "info"}
	module := newSnapshotReloadable(base)
	updated := map[string]any{"log.level": "debug"}
	diff := buildDiff(base, updated)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_ = manager.ApplyDiff(context.Background(), module, "app", diff)
		}()
	}
	close(start)
	wg.Wait()

	// Final value must be "debug" (no torn writes) and at least one application happened
	assert.Equal(t, "debug", module.Read("log.level"))
	assert.GreaterOrEqual(t, atomic.LoadInt32(&module.applied), int32(1))
}

func TestReloadAtomicFailureRollback(t *testing.T) {
	manager := NewReloadManager([]string{"log.level"})
	base := map[string]any{"log.level": "info"}
	module := newSnapshotReloadable(base)
	bad := map[string]any{"log.level": "fail"}
	diff := buildDiff(base, bad)
	err := manager.ApplyDiff(context.Background(), module, "app", diff)
	assert.Error(t, err)
	// value unchanged
	assert.Equal(t, "info", module.Read("log.level"))
}

func TestReloadTimeoutHonored(t *testing.T) {
	// Custom module with long timeout to verify context/cancellation path
	module := &delayedReloadable{delay: 50 * time.Millisecond}
	manager := NewReloadManager([]string{"log.level"})
	base := map[string]any{"log.level": "info"}
	updated := map[string]any{"log.level": "debug"}
	diff := buildDiff(base, updated)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := manager.ApplyDiff(ctx, module, "app", diff)
	assert.Error(t, err)
}

// delayedReloadable simulates a reload that respects context cancellation.
type delayedReloadable struct { delay time.Duration }
func (d *delayedReloadable) Reload(ctx context.Context, changes []modular.ConfigChange) error {
	select {
	case <-time.After(d.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (d *delayedReloadable) CanReload() bool { return true }
func (d *delayedReloadable) ReloadTimeout() time.Duration { return 5 * time.Millisecond }

func TestReloadHighFrequencyQueueing(t *testing.T) {
	manager := NewReloadManager([]string{"log.level"})
	base := map[string]any{"log.level": "info"}
	module := newSnapshotReloadable(base)
	diff := buildDiff(base, map[string]any{"log.level": "debug"})
	for i := 0; i < 100; i++ {
		_ = manager.ApplyDiff(context.Background(), module, "app", diff)
	}
	assert.Equal(t, "debug", module.Read("log.level"))
}

func TestReloadSnapshotVisibility(t *testing.T) {
	manager := NewReloadManager([]string{"log.level"})
	base := map[string]any{"log.level": "info"}
	module := newSnapshotReloadable(base)
	diff := buildDiff(base, map[string]any{"log.level": "debug"})
	err := manager.ApplyDiff(context.Background(), module, "app", diff)
	require.NoError(t, err)
	assert.Equal(t, "debug", module.Read("log.level"))
}
