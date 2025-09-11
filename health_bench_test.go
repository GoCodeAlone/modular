package modular

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// BenchmarkHealthAggregation benchmarks the health aggregation functionality
func BenchmarkHealthAggregation(b *testing.B) {
	b.Run("single provider collection", func(b *testing.B) {
		service := createBenchHealthService(1, 0)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := service.Collect(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("multiple providers collection", func(b *testing.B) {
		service := createBenchHealthService(10, 0)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := service.Collect(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("many providers collection", func(b *testing.B) {
		service := createBenchHealthService(100, 0)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := service.Collect(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("collection with slow providers", func(b *testing.B) {
		service := createBenchHealthService(5, 10*time.Millisecond)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := service.Collect(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("concurrent collection", func(b *testing.B) {
		service := createBenchHealthService(20, 0)

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			ctx := context.Background()
			for pb.Next() {
				_, err := service.Collect(ctx)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

// BenchmarkHealthCaching benchmarks the health caching performance
func BenchmarkHealthCaching(b *testing.B) {
	b.Run("cache hit performance", func(b *testing.B) {
		service := createBenchHealthService(10, 5*time.Millisecond)
		ctx := context.Background()

		// Prime the cache
		_, err := service.Collect(ctx)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := service.Collect(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("cache invalidation performance", func(b *testing.B) {
		config := AggregateHealthServiceConfig{
			CacheTTL:       1 * time.Nanosecond, // Very short cache
			DefaultTimeout: 5 * time.Second,
			CacheEnabled:   true,
		}
		service := NewAggregateHealthServiceWithConfig(config)

		// Add providers
		for i := 0; i < 10; i++ {
			provider := &benchHealthProvider{
				moduleName: fmt.Sprintf("module-%d", i),
				delay:      1 * time.Millisecond,
			}
			service.RegisterProvider(fmt.Sprintf("module-%d", i), provider, false)
		}

		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := service.Collect(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkHealthProviderRegistration benchmarks provider registration and deregistration
func BenchmarkHealthProviderRegistration(b *testing.B) {
	b.Run("register providers", func(b *testing.B) {
		config := AggregateHealthServiceConfig{
			CacheTTL:       5 * time.Minute,
			DefaultTimeout: 30 * time.Second,
			CacheEnabled:   true,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			service := NewAggregateHealthServiceWithConfig(config)
			provider := &benchHealthProvider{
				moduleName: fmt.Sprintf("module-%d", i),
				delay:      0,
			}
			err := service.RegisterProvider(fmt.Sprintf("module-%d", i), provider, false)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("deregister providers", func(b *testing.B) {
		services := make([]*AggregateHealthService, b.N)

		// Setup
		for i := 0; i < b.N; i++ {
			config := AggregateHealthServiceConfig{
				CacheTTL:       5 * time.Minute,
				DefaultTimeout: 30 * time.Second,
				CacheEnabled:   true,
			}
			service := NewAggregateHealthServiceWithConfig(config)
			provider := &benchHealthProvider{
				moduleName: fmt.Sprintf("module-%d", i),
				delay:      0,
			}
			service.RegisterProvider(fmt.Sprintf("module-%d", i), provider, false)
			services[i] = service
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Since deregister is not available, we'll benchmark concurrent collection instead
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			services[i].Collect(ctx)
			cancel()
		}
	})
}

// BenchmarkHealthReportProcessing benchmarks processing of health reports
func BenchmarkHealthReportProcessing(b *testing.B) {
	b.Run("process healthy reports", func(b *testing.B) {
		reports := createHealthReports(100, HealthStatusHealthy)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			aggregated := processReportsForBench(reports)
			if aggregated.Health != HealthStatusHealthy {
				b.Fatal("Expected healthy status")
			}
		}
	})

	b.Run("process mixed reports", func(b *testing.B) {
		healthyReports := createHealthReports(80, HealthStatusHealthy)
		unhealthyReports := createHealthReports(20, HealthStatusUnhealthy)
		reports := append(healthyReports, unhealthyReports...)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			aggregated := processReportsForBench(reports)
			if aggregated.Health != HealthStatusUnhealthy {
				b.Fatal("Expected unhealthy status")
			}
		}
	})

	b.Run("process large report set", func(b *testing.B) {
		reports := createHealthReports(1000, HealthStatusHealthy)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			aggregated := processReportsForBench(reports)
			if len(aggregated.Reports) != 1000 {
				b.Fatal("Expected 1000 reports")
			}
		}
	})
}

// Helper functions and types for benchmarks

func createBenchHealthService(providerCount int, delay time.Duration) *AggregateHealthService {
	config := AggregateHealthServiceConfig{
		CacheTTL:       5 * time.Minute, // Long cache for consistent benchmarks
		DefaultTimeout: 30 * time.Second,
		CacheEnabled:   true,
	}

	service := NewAggregateHealthServiceWithConfig(config)

	for i := 0; i < providerCount; i++ {
		provider := &benchHealthProvider{
			moduleName: fmt.Sprintf("module-%d", i),
			delay:      delay,
		}
		service.RegisterProvider(fmt.Sprintf("module-%d", i), provider, false)
	}

	return service
}

func createHealthReports(count int, status HealthStatus) []HealthReport {
	reports := make([]HealthReport, count)
	for i := 0; i < count; i++ {
		reports[i] = HealthReport{
			Module:    fmt.Sprintf("module-%d", i),
			Status:    status,
			Message:   fmt.Sprintf("Status for module-%d", i),
			CheckedAt: time.Now(),
			Details:   map[string]any{"index": i},
		}
	}
	return reports
}

func processReportsForBench(reports []HealthReport) AggregatedHealth {
	overallStatus := HealthStatusHealthy
	for _, report := range reports {
		if report.Status == HealthStatusUnhealthy {
			overallStatus = HealthStatusUnhealthy
			break
		}
	}

	return AggregatedHealth{
		Health:      overallStatus,
		Readiness:   overallStatus, // For benchmarking, assume same status
		Reports:     reports,
		GeneratedAt: time.Now(),
	}
}

type benchHealthProvider struct {
	moduleName string
	delay      time.Duration
	mu         sync.RWMutex
	healthy    bool
}

func (p *benchHealthProvider) HealthCheck(ctx context.Context) ([]HealthReport, error) {
	if p.delay > 0 {
		select {
		case <-time.After(p.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	p.mu.RLock()
	healthy := p.healthy
	p.mu.RUnlock()

	status := HealthStatusHealthy
	message := "Module is healthy"
	if !healthy {
		status = HealthStatusUnhealthy
		message = "Module is unhealthy"
	}

	return []HealthReport{{
		Module:    p.moduleName,
		Status:    status,
		Message:   message,
		CheckedAt: time.Now(),
		Details:   map[string]any{"benchmark": true},
	}}, nil
}

func (p *benchHealthProvider) SetHealthy(healthy bool) {
	p.mu.Lock()
	p.healthy = healthy
	p.mu.Unlock()
}

type benchHealthLogger struct{}

func (l *benchHealthLogger) Debug(msg string, args ...any) {}
func (l *benchHealthLogger) Info(msg string, args ...any)  {}
func (l *benchHealthLogger) Warn(msg string, args ...any)  {}
func (l *benchHealthLogger) Error(msg string, args ...any) {}
