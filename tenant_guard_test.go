package modular

import (
	"bytes"
	"context"
	"errors"
	"log"
	"sync"
	"testing"
	"time"
)

func TestTenantGuardMode_String(t *testing.T) {
	tests := []struct {
		mode TenantGuardMode
		want string
	}{
		{TenantGuardStrict, "strict"},
		{TenantGuardLenient, "lenient"},
		{TenantGuardDisabled, "disabled"},
		{TenantGuardMode(99), "unknown(99)"},
	}

	for _, tt := range tests {
		got := tt.mode.String()
		if got != tt.want {
			t.Errorf("TenantGuardMode(%d).String() = %q, want %q", int(tt.mode), got, tt.want)
		}
	}
}

func TestViolationType_String(t *testing.T) {
	tests := []struct {
		vt   ViolationType
		want string
	}{
		{CrossTenant, "cross_tenant"},
		{InvalidContext, "invalid_context"},
		{MissingContext, "missing_context"},
		{Unauthorized, "unauthorized"},
		{ViolationType(99), "unknown(99)"},
	}

	for _, tt := range tests {
		got := tt.vt.String()
		if got != tt.want {
			t.Errorf("ViolationType(%d).String() = %q, want %q", int(tt.vt), got, tt.want)
		}
	}
}

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		sev  Severity
		want string
	}{
		{SeverityLow, "low"},
		{SeverityMedium, "medium"},
		{SeverityHigh, "high"},
		{SeverityCritical, "critical"},
		{Severity(99), "unknown(99)"},
	}

	for _, tt := range tests {
		got := tt.sev.String()
		if got != tt.want {
			t.Errorf("Severity(%d).String() = %q, want %q", int(tt.sev), got, tt.want)
		}
	}
}

func TestStandardTenantGuard_StrictMode(t *testing.T) {
	config := DefaultTenantGuardConfig()
	config.Mode = TenantGuardStrict
	guard := NewStandardTenantGuard(config)

	err := guard.ValidateAccess(context.Background(), TenantViolation{
		Type:     CrossTenant,
		Severity: SeverityHigh,
		TenantID: "tenant-1",
		TargetID: "tenant-2",
		Details:  "cross-tenant data access",
	})

	if err == nil {
		t.Fatal("expected error in strict mode, got nil")
	}
	if !errors.Is(err, ErrTenantIsolationViolation) {
		t.Errorf("expected ErrTenantIsolationViolation, got %v", err)
	}

	violations := guard.GetRecentViolations()
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation recorded, got %d", len(violations))
	}
	if violations[0].TenantID != "tenant-1" {
		t.Errorf("expected tenant-1, got %s", violations[0].TenantID)
	}
}

func TestStandardTenantGuard_LenientMode(t *testing.T) {
	config := DefaultTenantGuardConfig()
	config.Mode = TenantGuardLenient

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	guard := NewStandardTenantGuard(config, WithTenantGuardLogger(logger))

	err := guard.ValidateAccess(context.Background(), TenantViolation{
		Type:     CrossTenant,
		Severity: SeverityMedium,
		TenantID: "tenant-1",
		TargetID: "tenant-2",
		Details:  "lenient test",
	})

	if err != nil {
		t.Fatalf("expected nil error in lenient mode, got %v", err)
	}

	violations := guard.GetRecentViolations()
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation recorded, got %d", len(violations))
	}

	if buf.Len() == 0 {
		t.Error("expected log output for violation, got none")
	}
}

func TestStandardTenantGuard_DisabledMode(t *testing.T) {
	config := DefaultTenantGuardConfig()
	config.Mode = TenantGuardDisabled
	guard := NewStandardTenantGuard(config)

	err := guard.ValidateAccess(context.Background(), TenantViolation{
		Type:     CrossTenant,
		Severity: SeverityCritical,
		TenantID: "tenant-1",
		TargetID: "tenant-2",
	})

	if err != nil {
		t.Fatalf("expected nil error in disabled mode, got %v", err)
	}

	violations := guard.GetRecentViolations()
	if len(violations) != 0 {
		t.Errorf("expected 0 violations in disabled mode, got %d", len(violations))
	}
}

func TestStandardTenantGuard_Whitelist(t *testing.T) {
	config := DefaultTenantGuardConfig()
	config.Mode = TenantGuardStrict
	config.Whitelist = map[string][]string{
		"tenant-1": {"tenant-2", "tenant-3"},
	}
	guard := NewStandardTenantGuard(config)

	// Whitelisted access should succeed
	err := guard.ValidateAccess(context.Background(), TenantViolation{
		Type:     CrossTenant,
		Severity: SeverityHigh,
		TenantID: "tenant-1",
		TargetID: "tenant-2",
	})
	if err != nil {
		t.Fatalf("expected nil for whitelisted access, got %v", err)
	}

	// Non-whitelisted access should fail in strict mode
	err = guard.ValidateAccess(context.Background(), TenantViolation{
		Type:     CrossTenant,
		Severity: SeverityHigh,
		TenantID: "tenant-1",
		TargetID: "tenant-99",
	})
	if !errors.Is(err, ErrTenantIsolationViolation) {
		t.Errorf("expected ErrTenantIsolationViolation for non-whitelisted access, got %v", err)
	}

	// Only the non-whitelisted violation should be recorded
	violations := guard.GetRecentViolations()
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
}

func TestStandardTenantGuard_RingBuffer(t *testing.T) {
	config := DefaultTenantGuardConfig()
	config.Mode = TenantGuardLenient
	config.MaxViolations = 5
	config.LogViolations = false
	guard := NewStandardTenantGuard(config)

	// Add 8 violations to a buffer of size 5
	for i := 0; i < 8; i++ {
		_ = guard.ValidateAccess(context.Background(), TenantViolation{
			Type:     CrossTenant,
			Severity: SeverityLow,
			TenantID: "tenant-1",
			TargetID: "target-" + string(rune('A'+i)),
			Details:  "violation",
		})
	}

	violations := guard.GetRecentViolations()
	if len(violations) != 5 {
		t.Fatalf("expected 5 violations (buffer size), got %d", len(violations))
	}

	// Oldest should be violation index 3 (target-D), newest should be index 7 (target-H)
	expectedTargets := []string{"target-D", "target-E", "target-F", "target-G", "target-H"}
	for i, v := range violations {
		if v.TargetID != expectedTargets[i] {
			t.Errorf("violation[%d].TargetID = %q, want %q", i, v.TargetID, expectedTargets[i])
		}
	}
}

func TestStandardTenantGuard_GetRecentViolations_DeepCopy(t *testing.T) {
	config := DefaultTenantGuardConfig()
	config.Mode = TenantGuardLenient
	config.LogViolations = false
	guard := NewStandardTenantGuard(config)

	_ = guard.ValidateAccess(context.Background(), TenantViolation{
		Type:     CrossTenant,
		Severity: SeverityHigh,
		TenantID: "tenant-1",
		TargetID: "tenant-2",
		Details:  "original",
	})

	// Get a copy and modify it
	copy1 := guard.GetRecentViolations()
	copy1[0].Details = "modified"

	// Get another copy — it should still have the original value
	copy2 := guard.GetRecentViolations()
	if copy2[0].Details != "original" {
		t.Errorf("internal state was mutated: expected 'original', got %q", copy2[0].Details)
	}
}

func TestStandardTenantGuard_ConcurrentAccess(t *testing.T) {
	config := DefaultTenantGuardConfig()
	config.Mode = TenantGuardLenient
	config.MaxViolations = 100
	config.LogViolations = false
	guard := NewStandardTenantGuard(config)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = guard.ValidateAccess(context.Background(), TenantViolation{
				Type:     CrossTenant,
				Severity: SeverityLow,
				TenantID: "tenant-1",
				TargetID: "tenant-2",
			})
		}(i)
	}
	wg.Wait()

	violations := guard.GetRecentViolations()
	if len(violations) != 100 {
		t.Errorf("expected 100 violations from concurrent access, got %d", len(violations))
	}
}

func TestStandardTenantGuard_TimestampAutoSet(t *testing.T) {
	config := DefaultTenantGuardConfig()
	config.Mode = TenantGuardLenient
	config.LogViolations = false
	guard := NewStandardTenantGuard(config)

	before := time.Now()
	_ = guard.ValidateAccess(context.Background(), TenantViolation{
		Type:     MissingContext,
		Severity: SeverityMedium,
		TenantID: "tenant-1",
	})
	after := time.Now()

	violations := guard.GetRecentViolations()
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}

	ts := violations[0].Timestamp
	if ts.Before(before) || ts.After(after) {
		t.Errorf("timestamp %v not between %v and %v", ts, before, after)
	}
}

func TestStandardTenantGuard_GetMode(t *testing.T) {
	for _, mode := range []TenantGuardMode{TenantGuardStrict, TenantGuardLenient, TenantGuardDisabled} {
		config := DefaultTenantGuardConfig()
		config.Mode = mode
		guard := NewStandardTenantGuard(config)
		if guard.GetMode() != mode {
			t.Errorf("GetMode() = %v, want %v", guard.GetMode(), mode)
		}
	}
}

func TestStandardTenantGuard_DefaultMaxViolations(t *testing.T) {
	config := DefaultTenantGuardConfig()
	if config.MaxViolations != 1000 {
		t.Errorf("DefaultTenantGuardConfig().MaxViolations = %d, want 1000", config.MaxViolations)
	}
	if !config.LogViolations {
		t.Error("DefaultTenantGuardConfig().LogViolations should be true")
	}
	if config.Mode != TenantGuardStrict {
		t.Errorf("DefaultTenantGuardConfig().Mode = %v, want strict", config.Mode)
	}
}

// Verify StandardTenantGuard satisfies the TenantGuard interface
var _ TenantGuard = (*StandardTenantGuard)(nil)
