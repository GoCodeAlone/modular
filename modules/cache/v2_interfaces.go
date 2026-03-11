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
func (m *CacheModule) CollectMetrics(ctx context.Context) modular.ModuleMetrics {
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
// It applies configuration changes for DefaultTTL, MaxItems, and CleanupInterval.
func (m *CacheModule) Reload(_ context.Context, changes []modular.ConfigChange) error {
	for _, ch := range changes {
		switch ch.FieldPath {
		case "defaultTTL":
			if d, err := time.ParseDuration(ch.NewValue); err == nil {
				m.config.DefaultTTL = d
			}
		case "maxItems":
			var n int
			if _, err := fmt.Sscan(ch.NewValue, &n); err == nil && n > 0 {
				m.config.MaxItems = n
			}
		case "cleanupInterval":
			if d, err := time.ParseDuration(ch.NewValue); err == nil {
				m.config.CleanupInterval = d
			}
		}
	}
	return nil
}
