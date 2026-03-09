# Dynamic Reload Manager — Reimplementation Plan

> Previously implemented in GoCodeAlone/modular (v1.4.3). Dropped during reset to GoCodeAlone/modular upstream.
> This document captures the design for future reimplementation.

## Overview

The Dynamic Reload Manager enables live configuration reloading for modules that implement the `Reloadable` interface. It uses a channel-based request queue, atomic processing guards, an exponential backoff circuit breaker for failure resilience, and emits lifecycle events via the observer pattern. Reloads have atomic semantics: all modules apply or all roll back.

## Key Interfaces

```go
type Reloadable interface {
    Reload(ctx context.Context, changes []ConfigChange) error
    CanReload() bool
    ReloadTimeout() time.Duration
}

type ConfigChange struct {
    Section   string
    FieldPath string
    OldValue  any
    NewValue  any
    Source    string
}

type ConfigDiff struct {
    Changed   map[string]FieldChange
    Added     map[string]FieldChange
    Removed   map[string]FieldChange
    Timestamp time.Time
    DiffID    string
}

type FieldChange struct {
    OldValue         any
    NewValue         any
    FieldPath        string
    ChangeType       ChangeType // Added, Modified, Removed
    IsSensitive      bool
    ValidationResult error
}

type ReloadTrigger int

const (
    ReloadManual     ReloadTrigger = iota
    ReloadFileChange
    ReloadAPIRequest
    ReloadScheduled
)
```

## Architecture

**ReloadOrchestrator** is the central coordinator:
- Module registry: `map[string]Reloadable` behind `sync.RWMutex`
- Request queue: buffered channel (capacity 100) of `ReloadRequest`
- Processing flag: `atomic.Bool` with CAS to ensure single-flight processing
- Background goroutine drains the request queue

**Circuit breaker** with exponential backoff:
- Base delay: 2 seconds
- Max delay cap: 2 minutes
- Formula: `min(base * 2^(failures-1), cap)`
- Resets to zero on successful reload
- Rejects requests while circuit is open (returns error immediately)

**Atomic reload semantics**:
1. Compute `ConfigDiff` between old and new config
2. Filter modules by affected sections
3. Check `CanReload()` on each; abort if any critical module refuses
4. Apply changes to each module with per-module timeout from `ReloadTimeout()`
5. On first failure: roll back already-applied modules with reverse changes
6. Emit completion or failure event

**Events** (via existing observer/event bus):
- `ConfigReloadStarted{ReloadID, Trigger, Sections}`
- `ConfigReloadCompleted{ReloadID, Duration, ModulesReloaded}`
- `ConfigReloadFailed{ReloadID, Error, ModulesFailed}`
- `ConfigReloadNoop{ReloadID, Reason}` — emitted when diff has no changes

**ConfigDiff methods**:
- `HasChanges() bool` — true if any Changed/Added/Removed entries
- `FilterByPrefix(prefix) ConfigDiff` — returns subset matching field path prefix
- `RedactSensitiveFields() ConfigDiff` — replaces sensitive values with `"[REDACTED]"`
- `ChangeSummary() string` — human-readable summary of changes

**HealthEvaluationMetrics** tracks per-reload stats: components evaluated, failed, skipped, timed out, and identifies the slowest component.

## Implementation Checklist

- [ ] Define `Reloadable` interface
- [ ] Define `ConfigChange`, `ConfigDiff`, `FieldChange` structs
- [ ] Implement `ConfigDiff` methods (HasChanges, FilterByPrefix, RedactSensitiveFields, ChangeSummary)
- [ ] Define `ReloadTrigger` enum
- [ ] Implement `ReloadOrchestrator` with module registry and RWMutex
- [ ] Implement channel-based request queue (buffered, size 100)
- [ ] Implement atomic CAS processing guard
- [ ] Implement exponential backoff circuit breaker (base 2s, cap 2m, factor 2^(n-1))
- [ ] Implement atomic reload with rollback on failure
- [ ] Implement per-module timeout via `ReloadTimeout()` and context cancellation
- [ ] Define and emit reload lifecycle events
- [ ] Implement `HealthEvaluationMetrics` tracking
- [ ] Add `RequestReload(sections ...string)` to application interface
- [ ] Add `WithDynamicReload()` builder option
- [ ] Write unit tests: successful reload, partial failure + rollback, circuit breaker backoff
- [ ] Write concurrency tests: concurrent reload requests, CAS contention
- [ ] Write example: HTTP server with reloadable timeouts (read/write/idle) and non-reloadable address/port

## Notes

- Modules that return `CanReload() == false` are skipped, not treated as errors.
- Rollback applies reverse `ConfigChange` entries (swap Old/New) in reverse module order.
- The request queue drops requests when full (capacity 100) and returns an error to the caller.
- Circuit breaker state is internal to the orchestrator; not exposed to modules.
- Sensitive field detection can use a configurable list of field path patterns (e.g., `*password*`, `*secret*`).
