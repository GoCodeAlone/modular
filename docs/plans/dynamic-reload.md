# Dynamic Reload Manager — Revised Implementation Plan

> Reset from CrisisTextLine/modular upstream (2026-03-09). This revision reflects what already exists.

## Gap Analysis

**Already exists:**
- Observer pattern with CloudEvents (`observer.go`) — foundation for reload events
- Config field tracking (`config_field_tracking.go`) — `FieldPopulation`, `StructStateDiffer`
- Config providers with thread-safe variants (`config_provider.go`) — `ImmutableConfigProvider` (atomic.Value)
- Circuit breaker pattern in reverseproxy (`modules/reverseproxy/circuit_breaker.go`) — reference implementation
- `EventTypeConfigChanged` event constant
- Module interfaces: `Module`, `Configurable`, `Startable`, `Stoppable`, `DependencyAware`
- Builder pattern with `WithOnConfigLoaded()` option

**Must implement:**
- `Reloadable` interface (add to `module.go`)
- `ConfigChange`, `ConfigDiff`, `FieldChange` types
- `ReloadTrigger` enum
- `ReloadOrchestrator` with request queue, CAS guard, circuit breaker
- Atomic reload with rollback semantics
- Reload lifecycle events (4 new event types)
- `RequestReload()` on Application interface
- `WithDynamicReload()` builder option
- Tests

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
- Base delay: 2 seconds, max delay cap: 2 minutes
- Formula: `min(base * 2^(failures-1), cap)`
- Resets on successful reload, rejects while open

**Atomic reload semantics**:
1. Compute `ConfigDiff` between old and new config
2. Filter modules by affected sections
3. Check `CanReload()` on each; skip those returning false
4. Apply changes with per-module timeout from `ReloadTimeout()`
5. On failure: roll back already-applied modules with reverse changes
6. Emit completion or failure event

**Events** (add to observer.go):
- `EventTypeConfigReloadStarted`
- `EventTypeConfigReloadCompleted`
- `EventTypeConfigReloadFailed`
- `EventTypeConfigReloadNoop`

## Files

| Action | File | What |
|--------|------|------|
| Create | `reload.go` | ConfigChange, ConfigDiff, FieldChange, ReloadTrigger types + ConfigDiff methods |
| Modify | `module.go` | Add Reloadable interface |
| Create | `reload_orchestrator.go` | ReloadOrchestrator implementation |
| Modify | `observer.go` | Add 4 reload event type constants |
| Modify | `application.go` | Add RequestReload() method |
| Modify | `builder.go` | Add WithDynamicReload() option |
| Create | `reload_test.go` | Unit + concurrency tests |

## Implementation Checklist

- [ ] Define `Reloadable` interface in module.go
- [ ] Create reload.go with ConfigChange, ConfigDiff, FieldChange, ChangeType, ReloadTrigger
- [ ] Implement ConfigDiff methods: HasChanges, FilterByPrefix, RedactSensitiveFields, ChangeSummary
- [ ] Add 4 reload event constants to observer.go
- [ ] Implement ReloadOrchestrator with module registry + RWMutex
- [ ] Implement channel-based request queue (buffered, size 100)
- [ ] Implement atomic CAS processing guard
- [ ] Implement exponential backoff circuit breaker
- [ ] Implement atomic reload with rollback on failure
- [ ] Implement per-module timeout via context cancellation
- [ ] Emit reload lifecycle events via observer
- [ ] Add RequestReload() to Application interface + StdApplication
- [ ] Add WithDynamicReload() builder option
- [ ] Write unit tests: successful reload, partial failure + rollback, circuit breaker
- [ ] Write concurrency tests: concurrent requests, CAS contention

## Notes

- Modules returning `CanReload() == false` are skipped, not errors.
- Rollback applies reverse ConfigChange entries in reverse module order.
- Queue drops requests when full (capacity 100) and returns error.
- Circuit breaker state is internal to orchestrator; not exposed to modules.
- Sensitive field detection uses configurable field path patterns (e.g., `*password*`, `*secret*`).
