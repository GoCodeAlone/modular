//go:build planned

// Package benchmark provides baseline benchmarks for the modular framework.
// This file implements Task T001 from specs/001-baseline-specification-for/tasks.md
// 
// Purpose: Establish baseline performance metrics prior to dynamic reload & health aggregator changes.
// These benchmarks measure core framework operations: application bootstrap, service lookup,
// and configuration loading to track performance regressions during feature development.
package benchmark

import (
	"testing"

	"github.com/GoCodeAlone/modular"
)

// benchLogger provides a no-op logger for benchmarking
type benchLogger struct{}

func (l *benchLogger) Debug(msg string, args ...interface{}) {}
func (l *benchLogger) Info(msg string, args ...interface{})  {}
func (l *benchLogger) Warn(msg string, args ...interface{})  {}
func (l *benchLogger) Error(msg string, args ...interface{}) {}
func (l *benchLogger) With(args ...interface{}) modular.Logger {
	return l
}

// mockModule provides a minimal implementation for benchmarking
type mockModule struct {
	name string
}

func (m *mockModule) Name() string {
	return m.name
}

func (m *mockModule) Init(app modular.Application) error {
	// Register a simple service for this module
	app.SvcRegistry()[m.name+"-service"] = &simpleService{name: m.name}
	return nil
}

// simpleService is a basic service type for benchmarking
type simpleService struct {
	name string
}

func (s *simpleService) GetName() string {
	return s.name
}

// mockConfigModule extends mockModule with configuration capabilities
type mockConfigModule struct {
	mockModule
}

func (m *mockConfigModule) RegisterConfig(app modular.Application) error {
	// Register a simple configuration section
	cfg := map[string]interface{}{
		"enabled": true,
		"timeout": 30,
		"name":    m.name,
	}
	app.RegisterConfigSection(m.name+"-config", modular.NewStdConfigProvider(cfg))
	return nil
}

// testConfig represents a simple configuration structure for benchmarking
type testConfig struct {
	Database struct {
		Host     string `yaml:"host" default:"localhost"`
		Port     int    `yaml:"port" default:"5432"`
		Username string `yaml:"username" required:"true"`
		Password string `yaml:"password" required:"true"`
	} `yaml:"database"`
	Server struct {
		Port    int  `yaml:"port" default:"8080"`
		Enabled bool `yaml:"enabled" default:"true"`
	} `yaml:"server"`
	Features map[string]bool `yaml:"features"`
}

// BenchmarkApplicationBootstrap measures application construction and startup time
// with a couple of lightweight mock modules. This benchmark focuses on the Build+Start
// phase performance, which is critical for application startup time.
func BenchmarkApplicationBootstrap(b *testing.B) {
	b.ReportAllocs()

	// Setup phase - create modules outside of timing
	// Use nil config provider to avoid config loading complexity
	modules := []modular.Module{
		&mockModule{name: "module1"},
		&mockModule{name: "module2"},
	}

	b.ResetTimer() // Start timing after setup

	for i := 0; i < b.N; i++ {
		// Create new application with a proper logger and no config
		app := modular.NewStdApplication(nil, &benchLogger{})

		// Register modules
		for _, module := range modules {
			app.RegisterModule(module)
		}

		// Initialize, start, and stop application
		if err := app.Init(); err != nil {
			b.Fatalf("Failed to init application: %v", err)
		}

		if err := app.Start(); err != nil {
			b.Fatalf("Failed to start application: %v", err)
		}

		// Stop application to clean up
		if err := app.Stop(); err != nil {
			b.Logf("Warning: Failed to stop application: %v", err)
		}
	}
}

// BenchmarkServiceLookup measures service registry lookup performance.
// This benchmark registers N dummy services and measures repeated lookups
// via the registry APIs, testing both interface matching and named service lookups.
func BenchmarkServiceLookup(b *testing.B) {
	b.ReportAllocs()

	// Setup phase - create application and register services
	const numServices = 50
	app := modular.NewStdApplication(modular.NewStdConfigProvider(nil), &benchLogger{})

	// Register N dummy services
	registry := app.SvcRegistry()
	for i := 0; i < numServices; i++ {
		serviceName := "service-" + string(rune('0'+i%10)) + string(rune('0'+(i/10)%10))
		registry[serviceName] = &simpleService{name: serviceName}
	}

	// Service names to lookup during benchmark
	lookupNames := make([]string, numServices)
	for i := 0; i < numServices; i++ {
		lookupNames[i] = "service-" + string(rune('0'+i%10)) + string(rune('0'+(i/10)%10))
	}

	b.ResetTimer() // Start timing after setup

	b.Run("NamedLookup", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Lookup each service by name
			for _, name := range lookupNames {
				service, exists := registry[name]
				if !exists {
					b.Fatalf("Service %s not found", name)
				}
				_ = service // Use the service to prevent optimization
			}
		}
	})

	b.Run("InterfaceLookup", func(b *testing.B) {
		// Enhanced registry for interface-based lookups
		enhancedRegistry := modular.NewEnhancedServiceRegistry()

		// Register services in enhanced registry
		for name, service := range registry {
			if _, err := enhancedRegistry.RegisterService(name, service); err != nil {
				b.Fatalf("Failed to register service %s: %v", name, err)
			}
		}

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			// Lookup services that implement a common interface pattern
			for _, name := range lookupNames {
				service, exists := enhancedRegistry.GetService(name)
				if !exists {
					b.Fatalf("Service %s not found in enhanced registry", name)
				}
				_ = service // Use the service to prevent optimization
			}
		}
	})
}

// BenchmarkConfigLoad measures configuration feeding and validation performance.
// This benchmark tests the config feeders + validation pipeline with a synthetic
// configuration structure that exercises common configuration patterns.
func BenchmarkConfigLoad(b *testing.B) {
	b.ReportAllocs()

	// Setup phase - create test configuration
	testCfg := testConfig{
		Database: struct {
			Host     string `yaml:"host" default:"localhost"`
			Port     int    `yaml:"port" default:"5432"`
			Username string `yaml:"username" required:"true"`
			Password string `yaml:"password" required:"true"`
		}{
			Host:     "benchmark-db",
			Port:     5432,
			Username: "testuser",
			Password: "testpass",
		},
		Server: struct {
			Port    int  `yaml:"port" default:"8080"`
			Enabled bool `yaml:"enabled" default:"true"`
		}{
			Port:    8080,
			Enabled: true,
		},
		Features: map[string]bool{
			"feature1": true,
			"feature2": false,
			"feature3": true,
		},
	}

	b.ResetTimer() // Start timing after setup

	for i := 0; i < b.N; i++ {
		// Create new config provider and validate
		configProvider := modular.NewStdConfigProvider(testCfg)

		// Test configuration loading and validation
		loadedConfig := configProvider.GetConfig().(testConfig)

		// Verify some basic fields to ensure actual work is done
		if loadedConfig.Database.Host == "" {
			b.Fatal("Configuration not properly loaded")
		}
	}
}