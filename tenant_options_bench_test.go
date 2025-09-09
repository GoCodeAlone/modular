package modular

import (
	"context"
	"testing"
)

// BenchmarkTenantGuardValidate compares performance across modes.
func BenchmarkTenantGuardValidate(b *testing.B) {
	benchmark := func(mode TenantGuardMode) {
		cfg := NewDefaultTenantGuardConfig(mode)
		guard := &stdTenantGuard{config: cfg, violations: make([]*TenantViolation, 0)}
		v := &TenantViolation{RequestingTenant: "t1", AccessedResource: "t2/resource", ViolationType: TenantViolationCrossTenantAccess}
		ctx := context.Background()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := guard.ValidateAccess(ctx, v); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	}

	b.Run("strict", func(b *testing.B) { benchmark(TenantGuardModeStrict) })
	b.Run("lenient", func(b *testing.B) { benchmark(TenantGuardModeLenient) })
	b.Run("disabled", func(b *testing.B) { benchmark(TenantGuardModeDisabled) })
}
