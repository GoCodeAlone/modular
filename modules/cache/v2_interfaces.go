package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Compile-time interface checks.
var (
	_ modular.MetricsProvider = (*CacheModule)(nil)
	_ modular.Reloadable      = (*CacheModule)(nil)
)

// CollectMetrics implements modular.MetricsProvider.
// It delegates to the underlying CacheEngine's Stats method.
// Safe to call before Init (returns empty metrics when engine is nil).
func (m *CacheModule) CollectMetrics(ctx context.Context) modular.ModuleMetrics {
	if m.cacheEngine == nil {
		return modular.ModuleMetrics{Name: m.name, Values: map[string]float64{}}
	}
	return modular.ModuleMetrics{
		Name:   m.name,
		Values: m.cacheEngine.Stats(ctx),
	}
}

// CanReload implements modular.Reloadable.
func (m *CacheModule) CanReload() bool {
	return true
}

// ReloadTimeout implements modular.Reloadable.
func (m *CacheModule) ReloadTimeout() time.Duration {
	return 5 * time.Second
}

// Reload implements modular.Reloadable.
// It applies configuration changes for DefaultTTL and MaxItems.
// CleanupInterval is not reloadable since the cleanup ticker is already running.
// Config writes are protected by configMu (and the engine's mutex for MaxItems)
// to avoid data races with concurrent reads.
func (m *CacheModule) Reload(_ context.Context, changes []modular.ConfigChange) error {
	for _, ch := range changes {
		switch ch.FieldPath {
		case "defaultTTL":
			if d, err := time.ParseDuration(ch.NewValue); err == nil {
				m.configMu.Lock()
				m.config.DefaultTTL = d
				m.configMu.Unlock()
			}
		case "maxItems":
			var n int
			if _, err := fmt.Sscan(ch.NewValue, &n); err == nil && n > 0 {
				m.configMu.Lock()
				// MemoryCache reads MaxItems under its own mutex, so lock both.
				if mc, ok := m.cacheEngine.(*MemoryCache); ok {
					mc.mutex.Lock()
					m.config.MaxItems = n
					mc.mutex.Unlock()
				} else {
					m.config.MaxItems = n
				}
				m.configMu.Unlock()
			}
		}
	}
	return nil
}
