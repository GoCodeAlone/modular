# Aggregate Health Service — Revised Implementation Plan

> Reset from CrisisTextLine/modular upstream (2026-03-09). This revision reflects what already exists.

## Gap Analysis

**Already exists (~15%):**
- ReverseProxy `HealthChecker` with concurrent backend checks, events, debug endpoints (`modules/reverseproxy/health_checker.go`)
- Backend health events: `EventTypeBackendHealthy`, `EventTypeBackendUnhealthy` (`modules/reverseproxy/events.go`)
- Observer pattern with CloudEvents (`observer.go`) — event emission infrastructure
- ReverseProxy circuit breaker (`modules/reverseproxy/circuit_breaker.go`)
- Database BDD health check stubs (`modules/database/bdd_connections_test.go`)
- HTTP server health monitoring BDD stubs (`modules/httpserver/bdd_health_monitoring_test.go`)

**Must implement (entire core service is new):**
- `HealthStatus` enum (Unknown/Healthy/Degraded/Unhealthy)
- `HealthProvider` interface
- `HealthReport` and `AggregatedHealth` structs
- `AggregateHealthService` with provider registry, concurrent fan-out, caching
- Per-provider panic recovery
- Temporary error detection (→ Degraded)
- Provider adapters: Simple, Static, Composite
- Health events: `HealthEvaluatedEvent`, `HealthStatusChangedEvent`
- Cache with TTL + force refresh context key

## Key Interfaces

```go
type HealthStatus int
const (
    StatusUnknown   HealthStatus = iota
    StatusHealthy
    StatusDegraded
    StatusUnhealthy
)

type HealthProvider interface {
    HealthCheck(ctx context.Context) ([]HealthReport, error)
}

type HealthReport struct {
    Module        string
    Component     string
    Status        HealthStatus
    Message       string
    CheckedAt     time.Time
    ObservedSince time.Time
    Optional      bool
    Details       map[string]any
}

type AggregatedHealth struct {
    Readiness   HealthStatus
    Health      HealthStatus
    Reports     []HealthReport
    GeneratedAt time.Time
}
```

## Architecture

- Provider registry: `map[string]HealthProvider` behind `sync.RWMutex`
- Cache: single `AggregatedHealth` with TTL (default 250ms), invalidated on provider add/remove
- Force refresh: `context.WithValue(ctx, ForceHealthRefreshKey, true)`
- Concurrent collection: fan-out goroutines, per-provider panic recovery, channel-based results
- Aggregation: Readiness = worst non-optional, Health = worst all. Unknown → Unhealthy for aggregation
- Temporary errors (`interface{ Temporary() bool }`) → Degraded; other errors → Unhealthy

## Files

| Action | File | What |
|--------|------|------|
| Create | `health.go` | HealthStatus enum, HealthProvider, HealthReport, AggregatedHealth, provider adapters |
| Create | `health_service.go` | AggregateHealthService implementation |
| Modify | `observer.go` | Add EventTypeHealthEvaluated, EventTypeHealthStatusChanged |
| Create | `health_test.go` | Unit + concurrency + panic recovery tests |

## Implementation Checklist

- [ ] Define HealthStatus enum with String() and IsHealthy()
- [ ] Define HealthProvider interface
- [ ] Define HealthReport and AggregatedHealth structs
- [ ] Add health event constants to observer.go
- [ ] Implement AggregateHealthService with provider registry + RWMutex
- [ ] Implement concurrent fan-out collection with goroutines + channel
- [ ] Implement per-provider panic recovery (panic → Unhealthy with details)
- [ ] Implement aggregation logic (readiness = worst non-optional, health = worst all)
- [ ] Implement cache with TTL (250ms default) and force-refresh context key
- [ ] Implement cache invalidation on provider add/remove
- [ ] Implement NewSimpleHealthProvider adapter
- [ ] Implement NewStaticHealthProvider adapter
- [ ] Implement NewCompositeHealthProvider adapter
- [ ] Implement temporary error detection (Degraded vs Unhealthy)
- [ ] Emit HealthEvaluatedEvent after each aggregation
- [ ] Emit HealthStatusChangedEvent on status transitions only
- [ ] Write unit tests: single provider, multiple providers, optional vs required
- [ ] Write cache tests: hit, miss, invalidation, force refresh
- [ ] Write concurrency tests: parallel checks, registration during check
- [ ] Write panic recovery tests

## Notes

- 250ms cache TTL prevents health check storms while keeping results fresh.
- Panic recovery ensures one misbehaving provider cannot crash the health system.
- `ObservedSince` tracks when current status was first seen, enabling duration-based alerting.
- Optional providers affect Health but not Readiness.
- Module-specific providers (cache, database, eventbus, reverseproxy) are examples, not required for core.
