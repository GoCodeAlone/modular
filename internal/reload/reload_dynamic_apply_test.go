package reload

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dynamicTestReloadable records applied changes; can inject failure.
type dynamicTestReloadable struct {
	applied [][]modular.ConfigChange
	failAt  int // index to fail (-1 means never)
	mu      sync.Mutex
}

func (d *dynamicTestReloadable) Reload(ctx context.Context, changes []modular.ConfigChange) error {
	// Simulate validation before apply: if failAt in range -> error before recording
	if d.failAt >= 0 && d.failAt < len(changes) {
		return assert.AnError
	}
	d.mu.Lock(); defer d.mu.Unlock()
	// Append deep copy for safety
	batch := make([]modular.ConfigChange, len(changes))
	copy(batch, changes)
	d.applied = append(d.applied, batch)
	return nil
}
func (d *dynamicTestReloadable) CanReload() bool          { return true }
func (d *dynamicTestReloadable) ReloadTimeout() time.Duration { return time.Second }

func TestReloadDynamicApply(t *testing.T) {
	manager := NewReloadManager([]string{"log.level", "cache.ttl"})
	module := &dynamicTestReloadable{failAt: -1}

	base := map[string]any{"log": map[string]any{"level": "info"}, "cache": map[string]any{"ttl": 30}, "server": map[string]any{"port": 8080}}
	updated := map[string]any{"log": map[string]any{"level": "debug"}, "cache": map[string]any{"ttl": 60}, "server": map[string]any{"port": 9090}}
	diff, err := modular.GenerateConfigDiff(base, updated)
	require.NoError(t, err)

	// Apply diff: server.port change should be static -> rejection (ErrStaticFieldChange)
	err = manager.ApplyDiff(context.Background(), module, "app", diff)
	assert.ErrorIs(t, err, ErrStaticFieldChange)
	assert.Len(t, manager.AppliedBatches(), 0, "No batch applied due to static field")

	// Remove static change and re-diff
	updated2 := map[string]any{"log": map[string]any{"level": "debug"}, "cache": map[string]any{"ttl": 60}, "server": map[string]any{"port": 8080}}
	diff2, err := modular.GenerateConfigDiff(base, updated2)
	require.NoError(t, err)
	err = manager.ApplyDiff(context.Background(), module, "app", diff2)
	assert.NoError(t, err)
	batches := manager.AppliedBatches()
	assert.Len(t, batches, 1)
	assert.Equal(t, 2, len(batches[0]), "Two dynamic changes applied")
}

func TestReloadDynamicAtomicFailure(t *testing.T) {
	manager := NewReloadManager([]string{"log.level", "cache.ttl"})
	module := &dynamicTestReloadable{failAt: 1} // second change fails
	base := map[string]any{"log": map[string]any{"level": "info"}, "cache": map[string]any{"ttl": 30}}
	updated := map[string]any{"log": map[string]any{"level": "debug"}, "cache": map[string]any{"ttl": 60}}
	diff, err := modular.GenerateConfigDiff(base, updated)
	require.NoError(t, err)
	err = manager.ApplyDiff(context.Background(), module, "app", diff)
	assert.Error(t, err)
	assert.Len(t, manager.AppliedBatches(), 0, "Atomic failure should not apply changes")
}

func TestReloadDynamicNoop(t *testing.T) {
	manager := NewReloadManager([]string{"log.level"})
	module := &dynamicTestReloadable{}
	base := map[string]any{"log": map[string]any{"level": "info"}}
	same := map[string]any{"log": map[string]any{"level": "info"}}
	diff, err := modular.GenerateConfigDiff(base, same)
	require.NoError(t, err)
	assert.True(t, diff.IsEmpty())
	err = manager.ApplyDiff(context.Background(), module, "app", diff)
	assert.NoError(t, err)
	assert.Len(t, manager.AppliedBatches(), 0, "No batch for noop diff")
}

func TestReloadConcurrency(t *testing.T) {
	manager := NewReloadManager([]string{"log.level"})
	module := &dynamicTestReloadable{failAt: -1}
	base := map[string]any{"log": map[string]any{"level": "info"}}
	updated := map[string]any{"log": map[string]any{"level": "debug"}}
	diff, _ := modular.GenerateConfigDiff(base, updated)

	// Prime once to ensure baseline application success
	err := manager.ApplyDiff(context.Background(), module, "app", diff)
	require.NoError(t, err)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_ = manager.ApplyDiff(context.Background(), module, "app", diff)
		}()
	}
	close(start)
	wg.Wait()

	// Serialized manager allows only sequential application; concurrent attempts all serialize.
	batches := manager.AppliedBatches()
	assert.GreaterOrEqual(t, len(batches), 1)
	assert.LessOrEqual(t, len(batches), 11) // initial prime + goroutines
}

