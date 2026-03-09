# Aggregate Health Service — Reimplementation Plan

> Previously implemented in GoCodeAlone/modular (v1.4.3). Dropped during reset to GoCodeAlone/modular upstream.
> This document captures the design for future reimplementation.

## Overview

The Aggregate Health Service collects health reports from registered providers, aggregates them into readiness and overall health statuses, and caches results with a configurable TTL. It supports concurrent health checks with panic recovery, emits status change events, and provides adapter patterns for simple, static, and composite health providers.

## Key Interfaces

```go
type HealthStatus int

const (
    StatusUnknown   HealthStatus = iota
    StatusHealthy
    StatusDegraded
    StatusUnhealthy
)

func (s HealthStatus) String() string   { /* "unknown", "healthy", "degraded", "unhealthy" */ }
func (s HealthStatus) IsHealthy() bool  { return s == StatusHealthy }

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
    Readiness   HealthStatus   // Worst of non-optional providers only
    Health      HealthStatus   // Worst of all providers
    Reports     []HealthReport
    GeneratedAt time.Time
}
```

## Architecture

**AggregateHealthService** is the central coordinator:
- Provider registry: `map[string]HealthProvider` behind `sync.RWMutex`
- Cache: single `AggregatedHealth` with timestamp, TTL default 250ms
- Force refresh: context value key to bypass cache

**Aggregation rules**:
- **Readiness**: worst status among non-optional providers only. Used for load balancer probes.
- **Health**: worst status among all providers. Used for monitoring/alerting.
- Ordering: Healthy < Degraded < Unhealthy (higher = worse, worst wins).
- Unknown treated as Unhealthy for aggregation purposes.

**Concurrent collection**:
- Fan-out goroutines to all providers simultaneously
- Per-provider panic recovery (panic -> Unhealthy report with panic details)
- Results collected via channel, aggregated after all complete or context cancels
- Temporary errors (implementing `interface{ Temporary() bool }`) produce Degraded; other errors produce Unhealthy

**Caching**:
- Enabled by default, TTL 250ms
- Invalidated when providers are added or removed
- Force refresh via `context.WithValue(ctx, ForceHealthRefreshKey, true)`

**Provider adapters**:
```go
// Wrap a function as a provider
func NewSimpleHealthProvider(name string, fn func(ctx context.Context) (HealthStatus, string, error)) HealthProvider

// Fixed status, useful for testing or static components
func NewStaticHealthProvider(reports ...HealthReport) HealthProvider

// Combine multiple providers into one
func NewCompositeHealthProvider(providers ...HealthProvider) HealthProvider
```

**Events**:
- `HealthEvaluatedEvent{Metrics}` — emitted after each aggregation with `HealthEvaluationMetrics` (components evaluated, failed, avg response time, bottleneck component name + duration)
- `HealthStatusChangedEvent{Previous, Current, ChangedAt}` — emitted only when aggregated status transitions

**Module-specific implementations** (examples for built-in modules):
- **Cache**: connectivity check (Set/Get/Delete cycle), capacity reporting
- **Database**: connection pool stats, ping latency
- **EventBus**: publish test event, worker count vs expected
- **ReverseProxy**: backend reachability with per-backend circuit breaker

## Implementation Checklist

- [ ] Define `HealthStatus` enum with `String()` and `IsHealthy()`
- [ ] Define `HealthProvider` interface
- [ ] Define `HealthReport` and `AggregatedHealth` structs
- [ ] Implement `AggregateHealthService` with provider registry and RWMutex
- [ ] Implement concurrent fan-out health collection with goroutines and channel
- [ ] Implement per-provider panic recovery
- [ ] Implement aggregation logic (readiness = worst non-optional, health = worst all)
- [ ] Implement cache with TTL (default 250ms) and force-refresh context key
- [ ] Implement cache invalidation on provider add/remove
- [ ] Implement `NewSimpleHealthProvider` adapter
- [ ] Implement `NewStaticHealthProvider` adapter
- [ ] Implement `NewCompositeHealthProvider` adapter
- [ ] Define and emit `HealthEvaluatedEvent` with metrics
- [ ] Define and emit `HealthStatusChangedEvent` on transitions
- [ ] Implement temporary error detection (Degraded vs Unhealthy)
- [ ] Write unit tests: single provider, multiple providers, optional vs required aggregation
- [ ] Write unit tests: cache hit/miss/invalidation, force refresh
- [ ] Write concurrency tests: parallel health checks, provider registration during check
- [ ] Write panic recovery tests
- [ ] Implement module-specific health providers (cache, database, eventbus, reverseproxy) as examples

## Notes

- The 250ms cache TTL prevents health check storms under high request rates while keeping results fresh.
- Panic recovery ensures one misbehaving provider cannot crash the entire health system.
- `ObservedSince` in `HealthReport` tracks when the current status was first seen, enabling duration-based alerting.
- Optional providers affect `Health` but not `Readiness`, allowing non-critical components to degrade without failing readiness probes.
- The bottleneck detection in `HealthEvaluationMetrics` identifies the slowest provider to aid performance tuning.
