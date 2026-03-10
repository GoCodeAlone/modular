package modular

import (
	"fmt"
	"sync"
	"testing"
)

// TestConfig is a simple config struct for testing
type TestConfig struct {
	Host     string
	Port     int
	Tags     []string
	Metadata map[string]string
}

func TestStdConfigProvider(t *testing.T) {
	t.Run("returns same reference", func(t *testing.T) {
		cfg := &TestConfig{Host: "localhost", Port: 8080}
		provider := NewStdConfigProvider(cfg)

		cfg1 := provider.GetConfig().(*TestConfig)
		cfg2 := provider.GetConfig().(*TestConfig)

		// Should be the exact same pointer
		if cfg1 != cfg2 {
			t.Error("StdConfigProvider should return same reference")
		}

		// Modifications affect all consumers
		cfg1.Port = 9090
		if cfg2.Port != 9090 {
			t.Error("Modifications should affect all consumers")
		}
	})
}

func TestIsolatedConfigProvider(t *testing.T) {
	t.Run("returns independent copies", func(t *testing.T) {
		cfg := &TestConfig{
			Host:     "localhost",
			Port:     8080,
			Tags:     []string{"a", "b"},
			Metadata: map[string]string{"key": "value"},
		}
		provider := NewIsolatedConfigProvider(cfg)

		cfg1 := provider.GetConfig().(*TestConfig)
		cfg2 := provider.GetConfig().(*TestConfig)

		// Should be different pointers
		if cfg1 == cfg2 {
			t.Error("IsolatedConfigProvider should return different references")
		}

		// Modifications should NOT affect other copies
		cfg1.Port = 9090
		if cfg2.Port == 9090 {
			t.Error("Modifications should not affect other copies")
		}
		if cfg2.Port != 8080 {
			t.Errorf("Expected port 8080, got %d", cfg2.Port)
		}
	})

	t.Run("deep copies nested structures", func(t *testing.T) {
		cfg := &TestConfig{
			Host:     "localhost",
			Port:     8080,
			Tags:     []string{"a", "b"},
			Metadata: map[string]string{"key": "value"},
		}
		provider := NewIsolatedConfigProvider(cfg)

		cfg1 := provider.GetConfig().(*TestConfig)
		cfg2 := provider.GetConfig().(*TestConfig)

		// Modify slice in cfg1
		cfg1.Tags[0] = "modified"
		if cfg2.Tags[0] == "modified" {
			t.Error("Slice modifications should not affect other copies")
		}

		// Modify map in cfg1
		cfg1.Metadata["key"] = "modified"
		if cfg2.Metadata["key"] == "modified" {
			t.Error("Map modifications should not affect other copies")
		}
	})

	t.Run("original is not modified", func(t *testing.T) {
		cfg := &TestConfig{Host: "localhost", Port: 8080}
		provider := NewIsolatedConfigProvider(cfg)

		copy := provider.GetConfig().(*TestConfig)
		copy.Port = 9090
		copy.Host = "example.com"

		// Original should remain unchanged
		if cfg.Port != 8080 {
			t.Errorf("Original port should be 8080, got %d", cfg.Port)
		}
		if cfg.Host != "localhost" {
			t.Errorf("Original host should be localhost, got %s", cfg.Host)
		}
	})
}

func TestImmutableConfigProvider(t *testing.T) {
	t.Run("returns same reference from atomic value", func(t *testing.T) {
		cfg := &TestConfig{Host: "localhost", Port: 8080}
		provider := NewImmutableConfigProvider(cfg)

		cfg1 := provider.GetConfig().(*TestConfig)
		cfg2 := provider.GetConfig().(*TestConfig)

		// Should be the same pointer (same atomic value)
		if cfg1 != cfg2 {
			t.Error("Should return same reference before update")
		}
	})

	t.Run("atomic update changes returned value", func(t *testing.T) {
		cfg := &TestConfig{Host: "localhost", Port: 8080}
		provider := NewImmutableConfigProvider(cfg)

		cfg1 := provider.GetConfig().(*TestConfig)

		// Update with new config
		newCfg := &TestConfig{Host: "example.com", Port: 443}
		provider.UpdateConfig(newCfg)

		cfg2 := provider.GetConfig().(*TestConfig)

		// Should now return the new config
		if cfg2.Host != "example.com" || cfg2.Port != 443 {
			t.Error("Should return updated config")
		}

		// Old reference should still have old values
		if cfg1.Host != "localhost" || cfg1.Port != 8080 {
			t.Error("Old reference should be unchanged")
		}
	})

	t.Run("concurrent reads are safe", func(t *testing.T) {
		cfg := &TestConfig{Host: "localhost", Port: 8080}
		provider := NewImmutableConfigProvider(cfg)

		var wg sync.WaitGroup
		errors := make(chan error, 100)

		// 100 concurrent readers
		for range 100 {
			wg.Go(func() {
				cfg := provider.GetConfig().(*TestConfig)
				if cfg == nil {
					errors <- fmt.Errorf("config is nil")
				}
			})
		}

		wg.Wait()
		close(errors)

		if len(errors) > 0 {
			t.Errorf("Concurrent reads failed with %d errors", len(errors))
		}
	})

	t.Run("concurrent reads during updates", func(t *testing.T) {
		cfg := &TestConfig{Host: "localhost", Port: 8080}
		provider := NewImmutableConfigProvider(cfg)

		var wg sync.WaitGroup
		errors := make(chan error, 100)

		// 50 concurrent readers
		for range 50 {
			wg.Go(func() {
				for range 100 {
					cfg := provider.GetConfig().(*TestConfig)
					if cfg == nil {
						errors <- ErrConfigNil
						return
					}
				}
			})
		}

		// 10 concurrent updaters
		for i := range 10 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := range 10 {
					newCfg := &TestConfig{
						Host: "example.com",
						Port: 8080 + id*100 + j,
					}
					provider.UpdateConfig(newCfg)
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		if len(errors) > 0 {
			t.Errorf("Concurrent operations failed with %d errors", len(errors))
		}
	})
}

func TestCopyOnWriteConfigProvider(t *testing.T) {
	t.Run("GetConfig returns original reference", func(t *testing.T) {
		cfg := &TestConfig{Host: "localhost", Port: 8080}
		provider := NewCopyOnWriteConfigProvider(cfg)

		cfg1 := provider.GetConfig().(*TestConfig)
		cfg2 := provider.GetConfig().(*TestConfig)

		// Should be the same reference
		if cfg1 != cfg2 {
			t.Error("GetConfig should return same reference")
		}
	})

	t.Run("GetMutableConfig returns independent copy", func(t *testing.T) {
		cfg := &TestConfig{
			Host:     "localhost",
			Port:     8080,
			Tags:     []string{"a", "b"},
			Metadata: map[string]string{"key": "value"},
		}
		provider := NewCopyOnWriteConfigProvider(cfg)

		original := provider.GetConfig().(*TestConfig)
		mutable, err := provider.GetMutableConfig()
		if err != nil {
			t.Fatalf("GetMutableConfig failed: %v", err)
		}

		mutableCfg := mutable.(*TestConfig)

		// Should be different pointers
		if original == mutableCfg {
			t.Error("GetMutableConfig should return different reference")
		}

		// Modifications should not affect original
		mutableCfg.Port = 9090
		mutableCfg.Tags[0] = "modified"
		mutableCfg.Metadata["key"] = "modified"

		if original.Port != 8080 {
			t.Error("Original should not be modified")
		}
		if original.Tags[0] != "a" {
			t.Error("Original slice should not be modified")
		}
		if original.Metadata["key"] != "value" {
			t.Error("Original map should not be modified")
		}
	})

	t.Run("UpdateOriginal changes GetConfig result", func(t *testing.T) {
		cfg := &TestConfig{Host: "localhost", Port: 8080}
		provider := NewCopyOnWriteConfigProvider(cfg)

		cfg1 := provider.GetConfig().(*TestConfig)

		// Update original
		newCfg := &TestConfig{Host: "example.com", Port: 443}
		provider.UpdateOriginal(newCfg)

		cfg2 := provider.GetConfig().(*TestConfig)

		// Should return new config
		if cfg2.Host != "example.com" || cfg2.Port != 443 {
			t.Error("Should return updated config")
		}

		// Old reference unchanged
		if cfg1.Host != "localhost" {
			t.Error("Old reference should be unchanged")
		}
	})

	t.Run("concurrent reads are safe", func(t *testing.T) {
		cfg := &TestConfig{Host: "localhost", Port: 8080}
		provider := NewCopyOnWriteConfigProvider(cfg)

		var wg sync.WaitGroup
		errors := make(chan error, 100)

		// 100 concurrent readers
		for range 100 {
			wg.Go(func() {
				cfg := provider.GetConfig().(*TestConfig)
				if cfg == nil {
					errors <- fmt.Errorf("config is nil")
				}
			})
		}

		wg.Wait()
		close(errors)

		if len(errors) > 0 {
			t.Errorf("Concurrent reads failed with %d errors", len(errors))
		}
	})

	t.Run("concurrent mutable copies are safe", func(t *testing.T) {
		cfg := &TestConfig{
			Host:     "localhost",
			Port:     8080,
			Tags:     []string{"a", "b"},
			Metadata: map[string]string{"key": "value"},
		}
		provider := NewCopyOnWriteConfigProvider(cfg)

		var wg sync.WaitGroup
		errors := make(chan error, 50)

		// 50 concurrent mutable copy requests
		for i := range 50 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				mutable, err := provider.GetMutableConfig()
				if err != nil {
					errors <- err
					return
				}
				mutableCfg := mutable.(*TestConfig)
				mutableCfg.Port = 8080 + id
			}(i)
		}

		wg.Wait()
		close(errors)

		if len(errors) > 0 {
			t.Errorf("Concurrent mutable copies failed with %d errors", len(errors))
		}

		// Original should be unchanged
		original := provider.GetConfig().(*TestConfig)
		if original.Port != 8080 {
			t.Errorf("Original port should be 8080, got %d", original.Port)
		}
	})
}

func TestIsolatedConfigProvider_ErrorFallback(t *testing.T) {
	// Test the error fallback path in IsolatedConfigProvider.GetConfig()
	// when DeepCopyConfig returns an error (e.g., nil config)

	// Create provider with nil config - this will trigger error in DeepCopyConfig
	provider := &IsolatedConfigProvider{cfg: nil}

	// GetConfig should handle the error gracefully and return nil
	result := provider.GetConfig()
	if result != nil {
		t.Errorf("Expected GetConfig to return nil for nil config, got %v", result)
	}
}

func TestDeepCopyConfig(t *testing.T) {
	t.Run("copies primitives", func(t *testing.T) {
		cfg := &TestConfig{Host: "localhost", Port: 8080}
		copied, err := DeepCopyConfig(cfg)
		if err != nil {
			t.Fatalf("DeepCopyConfig failed: %v", err)
		}

		copiedCfg := copied.(*TestConfig)

		// Values should match
		if copiedCfg.Host != cfg.Host || copiedCfg.Port != cfg.Port {
			t.Error("Copied values should match original")
		}

		// Should be different pointer
		if copiedCfg == cfg {
			t.Error("Should be different pointer")
		}
	})

	t.Run("deep copies slices", func(t *testing.T) {
		cfg := &TestConfig{
			Host: "localhost",
			Port: 8080,
			Tags: []string{"a", "b", "c"},
		}
		copied, err := DeepCopyConfig(cfg)
		if err != nil {
			t.Fatalf("DeepCopyConfig failed: %v", err)
		}

		copiedCfg := copied.(*TestConfig)

		// Modify copy's slice
		copiedCfg.Tags[0] = "modified"

		// Original should be unchanged
		if cfg.Tags[0] != "a" {
			t.Error("Original slice should not be modified")
		}
	})

	t.Run("deep copies maps", func(t *testing.T) {
		cfg := &TestConfig{
			Host:     "localhost",
			Port:     8080,
			Metadata: map[string]string{"key1": "value1", "key2": "value2"},
		}
		copied, err := DeepCopyConfig(cfg)
		if err != nil {
			t.Fatalf("DeepCopyConfig failed: %v", err)
		}

		copiedCfg := copied.(*TestConfig)

		// Modify copy's map
		copiedCfg.Metadata["key1"] = "modified"
		copiedCfg.Metadata["key3"] = "new"

		// Original should be unchanged
		if cfg.Metadata["key1"] != "value1" {
			t.Error("Original map should not be modified")
		}
		if _, exists := cfg.Metadata["key3"]; exists {
			t.Error("Original map should not have new key")
		}
	})

	t.Run("handles nil config", func(t *testing.T) {
		_, err := DeepCopyConfig(nil)
		if err != ErrConfigNil {
			t.Error("Should return ErrConfigNil for nil config")
		}
	})
}

// Benchmarks for performance comparison
func BenchmarkConfigProviders(b *testing.B) {
	cfg := &TestConfig{
		Host:     "localhost",
		Port:     8080,
		Tags:     []string{"a", "b", "c"},
		Metadata: map[string]string{"key1": "value1", "key2": "value2"},
	}

	b.Run("StdConfigProvider", func(b *testing.B) {
		provider := NewStdConfigProvider(cfg)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = provider.GetConfig()
		}
	})

	b.Run("IsolatedConfigProvider", func(b *testing.B) {
		provider := NewIsolatedConfigProvider(cfg)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = provider.GetConfig()
		}
	})

	b.Run("ImmutableConfigProvider", func(b *testing.B) {
		provider := NewImmutableConfigProvider(cfg)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = provider.GetConfig()
		}
	})

	b.Run("CopyOnWriteConfigProvider_Read", func(b *testing.B) {
		provider := NewCopyOnWriteConfigProvider(cfg)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = provider.GetConfig()
		}
	})

	b.Run("CopyOnWriteConfigProvider_Mutable", func(b *testing.B) {
		provider := NewCopyOnWriteConfigProvider(cfg)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = provider.GetMutableConfig()
		}
	})
}

// Benchmark concurrent access
func BenchmarkConcurrentReads(b *testing.B) {
	cfg := &TestConfig{
		Host:     "localhost",
		Port:     8080,
		Tags:     []string{"a", "b", "c"},
		Metadata: map[string]string{"key1": "value1"},
	}

	b.Run("ImmutableConfigProvider", func(b *testing.B) {
		provider := NewImmutableConfigProvider(cfg)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = provider.GetConfig()
			}
		})
	})

	b.Run("CopyOnWriteConfigProvider", func(b *testing.B) {
		provider := NewCopyOnWriteConfigProvider(cfg)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = provider.GetConfig()
			}
		})
	})
}
