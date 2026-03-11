package cache

import (
	"context"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestCacheModule creates a CacheModule initialised with a memory engine for testing.
func newTestCacheModule(t *testing.T) (*CacheModule, context.Context) {
	t.Helper()
	module := NewModule().(*CacheModule)
	app := newMockApp()

	// Pre-register config with explicit values so struct-tag defaults are not needed.
	cfg := &CacheConfig{
		Engine:          "memory",
		DefaultTTL:      300 * time.Second,
		CleanupInterval: 60 * time.Second,
		MaxItems:        10000,
	}
	app.RegisterConfigSection(module.Name(), modular.NewStdConfigProvider(cfg))

	require.NoError(t, module.RegisterConfig(app)) // skips (already registered)
	require.NoError(t, module.Init(app))

	ctx := context.Background()
	require.NoError(t, module.Start(ctx))
	t.Cleanup(func() { _ = module.Stop(ctx) })

	return module, ctx
}

func TestCacheModule_CollectMetrics(t *testing.T) {
	t.Parallel()

	module, ctx := newTestCacheModule(t)

	// Add some items
	require.NoError(t, module.Set(ctx, "key1", "val1", time.Minute))
	require.NoError(t, module.Set(ctx, "key2", "val2", time.Minute))
	require.NoError(t, module.Set(ctx, "key3", "val3", time.Minute))

	metrics := module.CollectMetrics(ctx)
	assert.Equal(t, "cache", metrics.Name)
	assert.Equal(t, 3.0, metrics.Values["item_count"])
	assert.Equal(t, 10000.0, metrics.Values["max_items"])
}

func TestCacheModule_Reloadable(t *testing.T) {
	t.Parallel()

	module, ctx := newTestCacheModule(t)

	// Verify interface compliance
	var reloadable modular.Reloadable = module
	assert.True(t, reloadable.CanReload())
	assert.Equal(t, 5*time.Second, reloadable.ReloadTimeout())

	// Verify reload updates config (cleanupInterval is not reloadable)
	changes := []modular.ConfigChange{
		{FieldPath: "defaultTTL", NewValue: "600s"},
		{FieldPath: "maxItems", NewValue: "5000"},
	}
	require.NoError(t, reloadable.Reload(ctx, changes))

	assert.Equal(t, 600*time.Second, module.config.DefaultTTL)
	assert.Equal(t, 5000, module.config.MaxItems)
}
