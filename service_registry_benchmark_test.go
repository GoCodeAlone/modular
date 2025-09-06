package modular

import (
	"fmt"
	"reflect"
	"testing"
)

// benchmarkScales defines the registry sizes we'll benchmark.
var benchmarkScales = []int{10, 100, 1000, 10000}

// dummyService is a minimal struct used for benchmark registrations.
type dummyService struct{ id int }

// dummyModule implements Module minimally for benchmarking currentModule tracking.
type dummyModule struct{ name string }

func (m *dummyModule) Name() string                      { return m.name }
func (m *dummyModule) Description() string               { return "benchmark dummy module" }
func (m *dummyModule) Version() string                   { return "v0.0.0" }
func (m *dummyModule) Config() any                       { return nil }
func (m *dummyModule) ConfigReflectType() reflect.Type   { return nil }
func (m *dummyModule) Services() []ServiceProvider       { return nil }
func (m *dummyModule) Dependencies() []ServiceDependency { return nil }
func (m *dummyModule) Init(app Application) error        { return nil }
func (m *dummyModule) Start(app Application) error       { return nil }
func (m *dummyModule) Stop(app Application) error        { return nil }

// BenchmarkRegisterService measures cost of registering N distinct services.
func BenchmarkRegisterService(b *testing.B) {
	for _, n := range benchmarkScales {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				r := NewEnhancedServiceRegistry()
				// Simulate registrations from a module to exercise naming conflict logic occasionally.
				mod := &dummyModule{name: "bench"}
				r.SetCurrentModule(mod)
				for j := 0; j < n; j++ {
					// Introduce some repeated base names to trigger uniqueness path.
					base := "svc"
					if j%10 == 0 { // every 10th uses identical name to force conflict path
						base = "conflict"
					}
					_, err := r.RegisterService(fmt.Sprintf("%s-%d", base, j), &dummyService{id: j})
					if err != nil {
						b.Fatalf("registration failed: %v", err)
					}
				}
				r.ClearCurrentModule()
			}
		})
	}
}

// prepareRegistry pre-populates a registry with n services; returns registry and slice of lookup keys.
func prepareRegistry(n int) (*EnhancedServiceRegistry, []string) {
	r := NewEnhancedServiceRegistry()
	mod := &dummyModule{name: "bench"}
	r.SetCurrentModule(mod)
	keys := make([]string, 0, n)
	for j := 0; j < n; j++ {
		name := fmt.Sprintf("svc-%d", j)
		key, _ := r.RegisterService(name, &dummyService{id: j})
		keys = append(keys, key)
	}
	r.ClearCurrentModule()
	return r, keys
}

// BenchmarkGetService measures lookup performance for existing services.
func BenchmarkGetService(b *testing.B) {
	for _, n := range benchmarkScales {
		r, keys := prepareRegistry(n)
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			idx := 0
			for i := 0; i < b.N; i++ {
				// cycle through keys
				key := keys[idx]
				if _, ok := r.GetService(key); !ok {
					b.Fatalf("service %s not found", key)
				}
				idx++
				if idx == len(keys) {
					idx = 0
				}
			}
		})
	}
}

// BenchmarkGetService_Miss measures cost of failed lookups.
func BenchmarkGetService_Miss(b *testing.B) {
	r, _ := prepareRegistry(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := r.GetService("__does_not_exist__"); ok {
			b.Fatalf("unexpected hit")
		}
	}
}
