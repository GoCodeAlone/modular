package testutil

import (
	"os"
	"testing"
)

// fakeT captures cleanup callbacks so we can invoke them manually to assert restoration
// behavior without relying on real *testing.T execution order.
// We avoid mocking *testing.T; instead we verify Isolate by observing that
// cleanup runs at end of the test function scope (standard testing behavior).

func TestWithIsolatedGlobals_RestoresEnv(t *testing.T) {
	// Set initial MODULAR_ENV, ensure APP_ENV unset
	os.Setenv("MODULAR_ENV", "orig")
	os.Unsetenv("APP_ENV")

	WithIsolatedGlobals(func() {
		// mutate inside
		os.Setenv("MODULAR_ENV", "changed")
		os.Setenv("APP_ENV", "added")
		if v := os.Getenv("MODULAR_ENV"); v != "changed" {
			t.Fatalf("expected changed inside, got %s", v)
		}
		if v := os.Getenv("APP_ENV"); v != "added" {
			t.Fatalf("expected added inside, got %s", v)
		}
	})

	// After call original state should be restored
	if v := os.Getenv("MODULAR_ENV"); v != "orig" {
		t.Fatalf("expected MODULAR_ENV=orig after restore, got %s", v)
	}
	if _, ok := os.LookupEnv("APP_ENV"); ok {
		t.Fatalf("APP_ENV should be unset after restore")
	}
}

func TestIsolate_RestoresEnvAndLIFO(t *testing.T) {
	os.Setenv("MODULAR_ENV", "base")
	os.Unsetenv("APP_ENV")

	// Register assertion first so it runs last (cleanup order is LIFO)
	t.Cleanup(func() {
		if v := os.Getenv("MODULAR_ENV"); v != "base" {
			t.Fatalf("expected MODULAR_ENV=base after cleanup, got %s", v)
		}
		if _, ok := os.LookupEnv("APP_ENV"); ok {
			t.Fatalf("APP_ENV should be unset after cleanup")
		}
	})

	// First isolate snapshot
	Isolate(t)
	os.Setenv("MODULAR_ENV", "layer1")
	os.Setenv("APP_ENV", "val1")

	// Second isolate snapshot after mutation
	Isolate(t)
	os.Setenv("MODULAR_ENV", "layer2")
	os.Setenv("APP_ENV", "val2")
}
