package testutil

import (
	"os"
	"sync"
	"testing"
)

// WithIsolatedGlobals snapshots and restores selected global mutable state so the caller
// can safely run a test in parallel without leaking changes. It is intentionally minimal
// and can be extended as more global state is introduced.
func WithIsolatedGlobals(fn func()) {
	// Deprecated feeder isolation: tests should now use per-application SetConfigFeeders.
	// We retain only environment variable isolation here.

	trackedEnv := []string{"MODULAR_ENV", "APP_ENV"}
	envSnapshot := map[string]*string{}
	for _, k := range trackedEnv {
		if v, ok := os.LookupEnv(k); ok {
			val := v
			envSnapshot[k] = &val
		} else {
			envSnapshot[k] = nil
		}
	}

	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()

	defer func() {
		for k, v := range envSnapshot {
			if v == nil {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, *v)
			}
		}
	}()

	fn()
}

// Isolate provides a *testing.T integrated variant of WithIsolatedGlobals.
// It snapshots selected global mutable state and environment variables, then
// registers a t.Cleanup to restore them automatically. Safe to call multiple
// times in a test (last restore wins, executed LIFO by t.Cleanup). Use this
// at the top of tests that mutate modular.ConfigFeeders or tracked env vars
// so they can run with other tests in parallel.
func Isolate(t *testing.T) {
	t.Helper()

	// Deprecated feeder isolation removed; only environment isolation remains.
	trackedEnv := []string{"MODULAR_ENV", "APP_ENV"}
	envSnapshot := map[string]*string{}
	for _, k := range trackedEnv {
		if v, ok := os.LookupEnv(k); ok {
			vCopy := v
			envSnapshot[k] = &vCopy
		} else {
			envSnapshot[k] = nil
		}
	}

	t.Cleanup(func() {
		for k, v := range envSnapshot {
			if v == nil {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, *v)
			}
		}
	})
}
