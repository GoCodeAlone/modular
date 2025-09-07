//go:build planned

// Package benchmark provides baseline performance benchmarks for the modular framework.
//
// This file implements Task T001: Baseline Benchmark as specified in the performance testing tasks spec.
// These benchmarks establish baseline performance metrics prior to dynamic reload & health aggregator implementations.
//
// Benchmarks included:
//   - BenchmarkApplicationBootstrap: Measures application Build+Start time with mock modules
//   - BenchmarkServiceLookup: Benchmarks repeated service lookup by interface/name
//
// All benchmarks are designed to be lightweight with no external network dependencies
// and minimal sleeps (<5ms) to ensure reliable and repeatable performance measurements.
package benchmark

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/GoCodeAlone/modular"
)

// TestStorage is a simple interface for benchmarking service lookups
type TestStorage interface {
	Store(key, value string)
	Retrieve(key string) string
}

// MockStorage implements TestStorage for benchmarking
type MockStorage struct {
	data map[string]string
}

func (m *MockStorage) Store(key, value string) {
	if m.data == nil {
		m.data = make(map[string]string)
	}
	m.data[key] = value
}

func (m *MockStorage) Retrieve(key string) string {
	if m.data == nil {
		return ""
	}
	return m.data[key]
}

// BenchmarkModule is a minimal module for benchmarking application bootstrap
type BenchmarkModule struct {
	name         string
	serviceCount int
}

func (m *BenchmarkModule) Name() string {
	return m.name
}

func (m *BenchmarkModule) Init(app modular.Application) error {
	// Register multiple services to simulate realistic module behavior
	for i := 0; i < m.serviceCount; i++ {
		serviceName := fmt.Sprintf("%s-storage-%d", m.name, i)
		storage := &MockStorage{data: make(map[string]string)}
		storage.Store("benchmark", "value")
		
		if err := app.RegisterService(serviceName, storage); err != nil {
			return err
		}
	}
	return nil
}

// TestLogger implements modular.Logger for benchmarking (minimal overhead)
type TestLogger struct{}

func (l *TestLogger) Debug(msg string, fields ...any)   {}
func (l *TestLogger) Info(msg string, fields ...any)    {}
func (l *TestLogger) Warn(msg string, fields ...any)    {}
func (l *TestLogger) Error(msg string, fields ...any)   {}
func (l *TestLogger) Logger() any                       { return l }

// BenchmarkApplicationBootstrap measures the time to build and start an application
// with multiple modules. This benchmark simulates realistic application startup.
func BenchmarkApplicationBootstrap(b *testing.B) {
	b.ReportAllocs()
	
	// Prepare configuration and logger outside the benchmark
	cfg := &struct{}{}
	configProvider := modular.NewStdConfigProvider(cfg)
	logger := &TestLogger{}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Create application using builder pattern (Build + Start)
		app, err := modular.NewApplication(
			modular.WithConfigProvider(configProvider),
			modular.WithLogger(logger),
			modular.WithModules(
				&BenchmarkModule{name: "module1", serviceCount: 3},
				&BenchmarkModule{name: "module2", serviceCount: 2},
				&BenchmarkModule{name: "module3", serviceCount: 1},
			),
		)
		if err != nil {
			b.Fatalf("Failed to build application: %v", err)
		}
		
		// Initialize the application (equivalent to Build phase)
		if err := app.Init(); err != nil {
			b.Fatalf("Failed to initialize application: %v", err)
		}
		
		// Start the application
		if err := app.Start(); err != nil {
			b.Fatalf("Failed to start application: %v", err)
		}
		
		// Clean shutdown to avoid resource leaks
		if err := app.Stop(); err != nil {
			b.Logf("Warning: failed to stop application: %v", err)
		}
	}
}

// BenchmarkServiceLookup benchmarks repeated service lookup operations
// by both name and interface type to measure registry performance.
func BenchmarkServiceLookup(b *testing.B) {
	b.ReportAllocs()
	
	// Setup: Create application with many services
	cfg := &struct{}{}
	configProvider := modular.NewStdConfigProvider(cfg)
	logger := &TestLogger{}
	
	app := modular.NewStdApplication(configProvider, logger)
	
	// Register N services for lookup benchmarking
	const serviceCount = 100
	serviceNames := make([]string, serviceCount)
	
	for i := 0; i < serviceCount; i++ {
		serviceName := fmt.Sprintf("benchmark-service-%d", i)
		serviceNames[i] = serviceName
		storage := &MockStorage{data: make(map[string]string)}
		
		if err := app.RegisterService(serviceName, storage); err != nil {
			b.Fatalf("Failed to register service %s: %v", serviceName, err)
		}
	}
	
	// Initialize application to complete service registration
	if err := app.Init(); err != nil {
		b.Fatalf("Failed to initialize application: %v", err)
	}
	
	b.ResetTimer()
	
	// Benchmark service lookup by name
	b.Run("ByName", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			serviceName := serviceNames[i%serviceCount]
			var storage TestStorage
			if err := app.GetService(serviceName, &storage); err != nil {
				b.Fatalf("Failed to get service %s: %v", serviceName, err)
			}
			if storage == nil {
				b.Fatal("Retrieved service is nil")
			}
		}
	})
	
	// Benchmark service lookup by interface type
	b.Run("ByInterface", func(b *testing.B) {
		interfaceType := reflect.TypeOf((*TestStorage)(nil)).Elem()
		introspector := app.ServiceIntrospector()
		
		for i := 0; i < b.N; i++ {
			entries := introspector.GetServicesByInterface(interfaceType)
			if len(entries) == 0 {
				b.Fatal("No services found by interface")
			}
			// Access first service to ensure lookup is complete
			_ = entries[0].Service
		}
	})
	
	// Clean shutdown
	if err := app.Stop(); err != nil {
		b.Logf("Warning: failed to stop application: %v", err)
	}
}

// BenchmarkServiceLookupWithBatchRegistration benchmarks service lookup performance
// when services are registered in batches, simulating module initialization patterns.
func BenchmarkServiceLookupWithBatchRegistration(b *testing.B) {
	b.ReportAllocs()
	
	// Setup baseline application
	cfg := &struct{}{}
	configProvider := modular.NewStdConfigProvider(cfg)
	logger := &TestLogger{}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		app := modular.NewStdApplication(configProvider, logger)
		
		// Register services in batches (simulating module registration)
		const batchSize = 10
		const batchCount = 5
		
		for batch := 0; batch < batchCount; batch++ {
			for j := 0; j < batchSize; j++ {
				serviceName := fmt.Sprintf("batch-%d-service-%d", batch, j)
				storage := &MockStorage{data: make(map[string]string)}
				if err := app.RegisterService(serviceName, storage); err != nil {
					b.Fatalf("Failed to register service: %v", err)
				}
			}
		}
		
		if err := app.Init(); err != nil {
			b.Fatalf("Failed to initialize application: %v", err)
		}
		b.StartTimer()
		
		// Perform lookups across all registered services
		for batch := 0; batch < batchCount; batch++ {
			for j := 0; j < batchSize; j++ {
				serviceName := fmt.Sprintf("batch-%d-service-%d", batch, j)
				var storage TestStorage
				if err := app.GetService(serviceName, &storage); err != nil {
					b.Fatalf("Failed to get service: %v", err)
				}
			}
		}
		
		b.StopTimer()
		if err := app.Stop(); err != nil {
			b.Logf("Warning: failed to stop application: %v", err)
		}
	}
}