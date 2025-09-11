package modular

import (
	"testing"
)

// BenchmarkGenerateConfigDiff benchmarks the config diff generation functionality
func BenchmarkGenerateConfigDiff(b *testing.B) {
	b.Run("simple config diff", func(b *testing.B) {
		oldConfig := testConfig{
			DatabaseHost: "old-host",
			ServerPort:   8080,
			CacheTTL:     "5m",
		}

		newConfig := testConfig{
			DatabaseHost: "new-host",
			ServerPort:   9090,
			CacheTTL:     "10m",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := GenerateConfigDiff(oldConfig, newConfig)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("large config diff", func(b *testing.B) {
		oldConfig := createLargeTestConfig(false)
		newConfig := createLargeTestConfig(true)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := GenerateConfigDiff(oldConfig, newConfig)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("nested config diff", func(b *testing.B) {
		oldConfig := benchNestedTestConfig{
			Parent: testConfig{
				DatabaseHost: "parent-old-host",
				ServerPort:   8080,
				CacheTTL:     "5m",
			},
			Child: testConfig{
				DatabaseHost: "child-old-host",
				ServerPort:   3000,
				CacheTTL:     "2m",
			},
			Settings: map[string]interface{}{
				"timeout":   30,
				"retries":   3,
				"log_level": "info",
				"features":  []string{"auth", "cache"},
			},
		}

		newConfig := benchNestedTestConfig{
			Parent: testConfig{
				DatabaseHost: "parent-new-host",
				ServerPort:   9090,
				CacheTTL:     "10m",
			},
			Child: testConfig{
				DatabaseHost: "child-new-host",
				ServerPort:   4000,
				CacheTTL:     "3m",
			},
			Settings: map[string]interface{}{
				"timeout":   60,
				"retries":   5,
				"log_level": "debug",
				"features":  []string{"auth", "cache", "metrics"},
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := GenerateConfigDiff(oldConfig, newConfig)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("no changes", func(b *testing.B) {
		config := testConfig{
			DatabaseHost: "same-host",
			ServerPort:   8080,
			CacheTTL:     "5m",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := GenerateConfigDiff(config, config)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkParseConfigChanges benchmarks parsing of config changes
func BenchmarkParseConfigChanges(b *testing.B) {
	changes := []ConfigFieldChange{
		{FieldPath: "name", OldValue: "old-name", NewValue: "new-name"},
		{FieldPath: "port", OldValue: 8080, NewValue: 9090},
		{FieldPath: "enabled", OldValue: true, NewValue: false},
		{FieldPath: "new_field", OldValue: nil, NewValue: "new-value"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate filtering dynamic changes by iterating through the changes
		dynamicCount := 0
		for _, change := range changes {
			if change.FieldPath != "" {
				dynamicCount++
			}
		}
		if dynamicCount == 0 {
			// Expected for this test data since none have dynamic tags
		}
	}
}

// BenchmarkReflectionBasedDiffing benchmarks reflection-heavy operations
func BenchmarkReflectionBasedDiffing(b *testing.B) {
	b.Run("struct with many fields", func(b *testing.B) {
		oldConfig := createStructWithManyFields(false)
		newConfig := createStructWithManyFields(true)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := GenerateConfigDiff(oldConfig, newConfig)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("struct with deep nesting", func(b *testing.B) {
		oldConfig := createDeeplyNestedConfig(3, false)
		newConfig := createDeeplyNestedConfig(3, true)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := GenerateConfigDiff(oldConfig, newConfig)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Helper functions for benchmark data generation

func createLargeTestConfig(variant bool) testConfig {
	suffix := "old"
	port := 8080
	ttl := "5m"
	if variant {
		suffix = "new"
		port = 9090
		ttl = "10m"
	}

	return testConfig{
		DatabaseHost: "large-host-" + suffix,
		ServerPort:   port,
		CacheTTL:     ttl,
	}
}

func createStructWithManyFields(variant bool) ManyFieldsConfig {
	base := "old"
	if variant {
		base = "new"
	}

	return ManyFieldsConfig{
		Field1:  base + "-1",
		Field2:  base + "-2",
		Field3:  base + "-3",
		Field4:  base + "-4",
		Field5:  base + "-5",
		Field6:  base + "-6",
		Field7:  base + "-7",
		Field8:  base + "-8",
		Field9:  base + "-9",
		Field10: base + "-10",
		Field11: variant,
		Field12: variant,
		Field13: variant,
		Field14: variant,
		Field15: variant,
	}
}

func createDeeplyNestedConfig(depth int, variant bool) interface{} {
	if depth <= 0 {
		base := "leaf-old"
		if variant {
			base = "leaf-new"
		}
		return base
	}

	return map[string]interface{}{
		"level": depth,
		"data":  createDeeplyNestedConfig(depth-1, variant),
	}
}

// Test config structures for benchmarking

type benchNestedTestConfig struct {
	Parent   testConfig             `json:"parent"`
	Child    testConfig             `json:"child"`
	Settings map[string]interface{} `json:"settings"`
}

// testConfig is defined in config_diff_test.go

type ManyFieldsConfig struct {
	Field1  string `json:"field1" dynamic:"true"`
	Field2  string `json:"field2"`
	Field3  string `json:"field3" dynamic:"true"`
	Field4  string `json:"field4"`
	Field5  string `json:"field5" dynamic:"true"`
	Field6  string `json:"field6"`
	Field7  string `json:"field7" dynamic:"true"`
	Field8  string `json:"field8"`
	Field9  string `json:"field9" dynamic:"true"`
	Field10 string `json:"field10"`
	Field11 bool   `json:"field11" dynamic:"true"`
	Field12 bool   `json:"field12"`
	Field13 bool   `json:"field13" dynamic:"true"`
	Field14 bool   `json:"field14"`
	Field15 bool   `json:"field15" dynamic:"true"`
}
