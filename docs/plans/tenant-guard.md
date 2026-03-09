# TenantGuard Framework — Reimplementation Plan

> Previously implemented in GoCodeAlone/modular (v1.4.3). Dropped during reset to GoCodeAlone/modular upstream.
> This document captures the design for future reimplementation.

## Overview

TenantGuard provides multi-tenant isolation enforcement for the modular framework. It validates cross-tenant access at runtime with configurable strictness (strict/lenient/disabled), tracks violations with severity levels, and integrates with the application builder via decorator and builder option patterns. All tenant state is RWMutex-protected for concurrent access.

## Key Interfaces

```go
type TenantGuardMode int

const (
    TenantGuardStrict  TenantGuardMode = iota // Block cross-tenant access
    TenantGuardLenient                         // Allow but log violations
    TenantGuardDisabled                        // No enforcement
)

type TenantGuard interface {
    GetMode() TenantGuardMode
    ValidateAccess(ctx context.Context, violation TenantViolation) error
    GetRecentViolations() []TenantViolation
}

type TenantService interface {
    GetTenantConfig(tenantID string) (TenantConfig, error)
    GetTenants() []string
    RegisterTenant(tenantID string, config TenantConfig) error
    RegisterTenantAwareModule(module TenantAwareModule)
}

type TenantAwareModule interface {
    OnTenantRegistered(tenantID string, config TenantConfig)
    OnTenantRemoved(tenantID string)
}
```

```go
type TenantViolation struct {
    Type      ViolationType // CrossTenant, InvalidContext, MissingContext, Unauthorized
    Severity  Severity      // Low, Medium, High, Critical
    TenantID  string
    TargetID  string
    Timestamp time.Time
    Details   string
}

type TenantGuardConfig struct {
    Mode              TenantGuardMode
    EnforceIsolation  bool
    AllowCrossTenant  bool
    ValidationTimeout time.Duration
    CacheSize         int
    CacheTTL          time.Duration
    Whitelist         map[string][]string // tenantID -> allowed target tenant IDs
    LogViolations     bool
    BlockViolations   bool
}
```

## Architecture

**Context propagation**: `TenantContext` wraps `context.Context` with a tenant ID value. `GetTenantIDFromContext(ctx)` extracts it. All tenant-scoped operations must carry tenant context.

**Config isolation**: `TenantConfigProvider` stores per-tenant config sections behind an `RWMutex`. Config reads return deep copies to prevent mutation. `TenantAffixedEnvFeeder` loads environment variables with tenant-specific prefixes/suffixes (e.g., `TENANT_ACME_DB_HOST`).

**Decorator pattern**: `TenantAwareDecorator` wraps the application to inject tenant context into request processing. It intercepts module lifecycle calls and routes them through the tenant service.

**Concurrency model**: All mutable state (`violations` slice, `config` maps, `whitelist`) protected by `sync.RWMutex`. `GetRecentViolations()` returns a deep copy to prevent data races. Violation tracking uses a bounded ring buffer to cap memory.

**Error types**: Sentinel errors (`ErrTenantNotFound`, `ErrTenantConfigNotFound`, `ErrTenantIsolationViolation`, `ErrTenantContextMissing`) for typed error handling.

## Implementation Checklist

- [ ] Define `TenantGuardMode` enum with String() method
- [ ] Define `ViolationType` and `Severity` enums
- [ ] Implement `TenantViolation` struct with timestamp tracking
- [ ] Implement `TenantGuardConfig` with sane defaults
- [ ] Implement `TenantGuard` interface and default implementation with RWMutex-protected violation ring buffer
- [ ] Implement `TenantContext` with `context.WithValue` / `GetTenantIDFromContext()`
- [ ] Implement `TenantService` interface and default implementation
- [ ] Implement `TenantAwareModule` lifecycle hook dispatch (fan-out on register/remove)
- [ ] Implement `TenantConfigProvider` with per-tenant config sections and deep copy reads
- [ ] Implement `TenantAffixedEnvFeeder` for tenant-specific env var loading
- [ ] Implement `TenantAwareDecorator` application decorator
- [ ] Add builder options: `WithTenantGuardMode()`, `WithTenantGuardModeConfig()`, `WithTenantAware()`
- [ ] Define sentinel error types
- [ ] Write unit tests for all modes (strict blocks, lenient logs, disabled skips)
- [ ] Write concurrency tests (parallel ValidateAccess, concurrent tenant registration)

## Notes

- Whitelist map allows explicit cross-tenant access for service accounts or admin tenants.
- Violation buffer should be bounded (e.g., 1000 entries) to prevent unbounded memory growth.
- Strict mode returns an error from `ValidateAccess`; lenient mode logs and returns nil.
- `GetRecentViolations()` must deep-copy to avoid callers mutating internal state.
- Consider emitting events via the observer pattern for violation tracking integration.
