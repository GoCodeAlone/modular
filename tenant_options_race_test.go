package modular

import (
	"context"
	"sync"
	"testing"
)

// TestTenantGuardConcurrentValidate ensures ValidateAccess is safe under concurrent access.
// NOTE: Current implementation isn't synchronized; this test will expose any data race on violations slice.
func TestTenantGuardConcurrentValidate(t *testing.T) {
	cfg := NewDefaultTenantGuardConfig(TenantGuardModeLenient)
	guard := &stdTenantGuard{config: cfg, violations: make([]*TenantViolation, 0)}

	// We'll run many goroutines appending violations concurrently
	var wg sync.WaitGroup
	iterations := 200
	wg.Add(iterations)

	for i := 0; i < iterations; i++ {
		go func(id int) {
			defer wg.Done()
			v := &TenantViolation{RequestingTenant: "t1", AccessedResource: "t2/resource", ViolationType: TenantViolationCrossTenantAccess}
			allowed, err := guard.ValidateAccess(context.Background(), v)
			if err != nil {
				// unexpected error
				panic(err)
			}
			if !allowed {
				// lenient mode should always allow
				panic("lenient mode denied access")
			}
		}(i)
	}

	wg.Wait()

	// Basic sanity: some violations recorded
	if len(guard.GetRecentViolations()) == 0 {
		t.Fatalf("expected violations recorded")
	}
}
