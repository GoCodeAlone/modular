# TenantGuard Framework — Revised Implementation Plan

> Reset from CrisisTextLine/modular upstream (2026-03-09). This revision reflects what already exists.

## Gap Analysis

**Already exists (~50% complete):**
- `TenantContext` with context propagation (`tenant.go:51-94`)
- `TenantService` interface + `StandardTenantService` implementation (`tenant.go`, `tenant_service.go`)
- `TenantAwareModule` interface with lifecycle hooks (`tenant.go:211-230`)
- `TenantConfigProvider` with RWMutex, isolation, immutability variants (`tenant_config_provider.go`)
- `TenantConfigLoader` + file-based implementation (`tenant_config_loader.go`, `tenant_config_file_loader.go`)
- `TenantAwareConfig` context-aware resolution (`tenant_aware_config.go`)
- `TenantAwareDecorator` application decorator (`decorator_tenant.go`)
- `TenantAffixedEnvFeeder` for tenant-specific env vars (`feeders/tenant_affixed_env.go`)
- `WithTenantAware()` builder option (`builder.go:163-169`)
- 8 tenant sentinel errors in `errors.go`
- ~28 test files covering tenant basics

**Must implement:**
- `TenantGuard` interface + `StandardTenantGuard` implementation
- `TenantGuardMode` enum (Strict/Lenient/Disabled)
- `ViolationType` + `Severity` enums
- `TenantViolation` struct
- `TenantGuardConfig` with defaults
- Ring buffer for bounded violation history
- Whitelist support
- `WithTenantGuardMode()` + `WithTenantGuardModeConfig()` builder options
- 2 missing sentinel errors
- Violation event emission via observer
- Mode-specific tests + concurrency tests

## Key Types (new)

```go
type TenantGuardMode int
const (
    TenantGuardStrict  TenantGuardMode = iota
    TenantGuardLenient
    TenantGuardDisabled
)

type ViolationType int
const (
    CrossTenant ViolationType = iota
    InvalidContext
    MissingContext
    Unauthorized
)

type Severity int
const (
    SeverityLow Severity = iota
    SeverityMedium
    SeverityHigh
    SeverityCritical
)

type TenantViolation struct {
    Type      ViolationType
    Severity  Severity
    TenantID  string
    TargetID  string
    Timestamp time.Time
    Details   string
}

type TenantGuard interface {
    GetMode() TenantGuardMode
    ValidateAccess(ctx context.Context, violation TenantViolation) error
    GetRecentViolations() []TenantViolation
}

type TenantGuardConfig struct {
    Mode              TenantGuardMode
    EnforceIsolation  bool
    AllowCrossTenant  bool
    ValidationTimeout time.Duration
    Whitelist         map[string][]string
    MaxViolations     int
    LogViolations     bool
}
```

## Files

| Action | File | What |
|--------|------|------|
| Create | `tenant_guard.go` | TenantGuardMode, ViolationType, Severity enums, TenantViolation, TenantGuardConfig, TenantGuard interface, StandardTenantGuard with ring buffer |
| Modify | `errors.go` | Add ErrTenantContextMissing, ErrTenantIsolationViolation |
| Modify | `builder.go` | Add WithTenantGuardMode(), WithTenantGuardModeConfig() |
| Modify | `observer.go` | Add EventTypeTenantViolation constant |
| Create | `tenant_guard_test.go` | Unit + concurrency tests |

## Implementation Checklist

- [ ] Create tenant_guard.go with TenantGuardMode enum + String()
- [ ] Add ViolationType and Severity enums with String() methods
- [ ] Implement TenantViolation struct
- [ ] Implement TenantGuardConfig with defaults (MaxViolations: 1000, LogViolations: true)
- [ ] Implement StandardTenantGuard with RWMutex-protected ring buffer
- [ ] Implement ValidateAccess: strict returns error, lenient logs, disabled no-op
- [ ] Implement whitelist checking in ValidateAccess
- [ ] Implement GetRecentViolations with deep copy
- [ ] Add ErrTenantContextMissing and ErrTenantIsolationViolation to errors.go
- [ ] Add EventTypeTenantViolation to observer.go
- [ ] Add WithTenantGuardMode() and WithTenantGuardModeConfig() to builder.go
- [ ] Write tests: strict blocks, lenient logs, disabled skips
- [ ] Write tests: whitelist bypass, ring buffer FIFO eviction
- [ ] Write concurrency tests: parallel ValidateAccess, concurrent violations

## Notes

- Ring buffer bounded at MaxViolations (default 1000) entries; FIFO eviction when full.
- Strict mode returns ErrTenantIsolationViolation; lenient logs + returns nil.
- GetRecentViolations() deep-copies to prevent caller mutation.
- Whitelist allows explicit cross-tenant access for service accounts.
- Emit EventTypeTenantViolation via observer for external monitoring integration.
