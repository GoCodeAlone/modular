package modular

import (
    "context"
    "sync"
    "testing"
    "time"
)

// These additional tests focus on internal branches of stdTenantGuard not fully
// exercised by the primary option tests. They validate concurrent violation
// logging behavior and timestamp mutation performed inside logViolation.

func TestTenantGuard_LogViolationTimestampAndCopyIsolation(t *testing.T) {
    guard := &stdTenantGuard{config: TenantGuardConfig{Mode: TenantGuardModeLenient}}

    v := &TenantViolation{ViolationType: TenantViolationCrossTenantAccess}
    if _, err := guard.ValidateAccess(context.Background(), v); err != nil { // lenient mode logs
        t.Fatalf("unexpected error: %v", err)
    }
    violations := guard.GetRecentViolations()
    if len(violations) != 1 {
        t.Fatalf("expected 1 violation, got %d", len(violations))
    }
    if violations[0].Timestamp.IsZero() {
        t.Fatalf("expected timestamp to be set on violation")
    }
    // Ensure slice copy isolation (mutate returned slice and confirm guard internal not affected)
    copySlice := violations
    copySlice[0].AccessedResource = "tampered"
    internal := guard.GetRecentViolations()
    if internal[0].AccessedResource == "tampered" {
        t.Fatalf("mutation of returned slice should not affect internal slice")
    }
}

func TestTenantGuard_ConcurrentViolationLogging(t *testing.T) {
    guard := &stdTenantGuard{config: TenantGuardConfig{Mode: TenantGuardModeLenient}}
    var wg sync.WaitGroup
    iterations := 25
    ctx := context.Background()
    for i := 0; i < iterations; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            _, err := guard.ValidateAccess(ctx, &TenantViolation{ViolationType: TenantViolationCrossTenantAccess})
            if err != nil {
                t.Errorf("unexpected error: %v", err)
            }
        }()
    }
    wg.Wait()
    // Give the last timestamp writes a moment (should be instant, but be safe)
    time.Sleep(10 * time.Millisecond)
    v := guard.GetRecentViolations()
    if len(v) != iterations { // each access logged once
        t.Fatalf("expected %d violations, got %d", iterations, len(v))
    }
    for i, viol := range v {
        if viol.Timestamp.IsZero() {
            t.Fatalf("violation %d has zero timestamp", i)
        }
    }
}
