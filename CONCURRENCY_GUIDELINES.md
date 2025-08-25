# Concurrency & Data Race Guidelines

This document codifies the concurrency patterns adopted in the Modular framework to ensure deterministic, race-free behavior under the Go race detector while preserving clarity and performance.

## Design Principles
1. **Safety First**: Code MUST pass `go test -race` across core, modules, examples, and CLI. Any race is a release blocker.
2. **Clarity Over Cleverness**: Prefer simple, easily audited synchronization over intricate lock-free or channel gymnastics unless a measurable performance need is proven.
3. **Immutability by Construction**: When feasible, construct immutable snapshots (config, slices, maps, request bodies) and share read-only.
4. **Encapsulation**: Internal goroutines own their state; external callers interact via explicit update / retrieval APIs instead of mutating shared maps or slices directly.
5. **Minimize Lock Scope**: Hold locks only around mutation or snapshot creation—never across blocking I/O or user callbacks.

## Core Synchronization Toolkit
| Concern | Preferred Primitive | Rationale |
|---------|---------------------|-----------|
| Multiple readers, infrequent writers | `sync.RWMutex` | Cheap uncontended reads; explicit write exclusion |
| Single-owner background goroutine publishing snapshots | Atomic pointer swap to immutable struct/map | Zero-copy read, no per-read locking |
| Bounded append-only event capture in tests | Mutex around slice | Simplicity; channels add ordering complexity |
| Parallel fan-out needing shared input body | Pre-buffer into `[]byte` + pass slice | Eliminates per-goroutine `*http.Request` body races |
| Config maps provided by caller | Defensive deep copy under lock | Prevents external mutation races |

Avoid channels for mere mutual exclusion; use them when you model a queue, backpressure, or lifecycle signaling.

## Observer Pattern Standard
All framework + module observers follow this template:
```go
type Subject struct {
    mu        sync.RWMutex
    observers []Observer // immutable only while read-locked
}

func (s *Subject) Register(o Observer) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.observers = append(s.observers, o)
}

func (s *Subject) notify(evt Event) {
    // Snapshot under read lock, then fan out without holding lock
    s.mu.RLock()
    snapshot := make([]Observer, len(s.observers))
    copy(snapshot, s.observers)
    s.mu.RUnlock()
    for _, o := range snapshot { // each observer must be internally thread-safe
        o.OnEvent(evt)
    }
}
```
Key points:
- Lock only to mutate the slice or take a snapshot.
- Do *not* hold locks while invoking observers.
- Observers that buffer events (tests) protect their internal slices with a mutex.

## Defensive Copy Pattern (Configs & Maps)
When accepting maps/slices in constructors or update APIs:
```go
func NewHealthChecker(cfg *Config) *HealthChecker {
    hc := &HealthChecker{}
    hc.mu.Lock()
    hc.endpoints = cloneStringMap(cfg.HealthEndpoints)
    hc.backends = cloneBackendMap(cfg.Backends)
    hc.mu.Unlock()
    return hc
}

func (hc *HealthChecker) UpdateHealthConfig(hcCfg *HealthConfig) {
    hc.mu.Lock()
    hc.endpoints = cloneStringMap(hcCfg.HealthEndpoints)
    hc.mu.Unlock()
}
```
External code MUST NOT rely on mutating original maps after construction; changes go through explicit APIs.

## Request Body Pre-Read for Parallel Fan-Out
Problem: Parallel backends reading/mutating `*http.Request` body concurrently -> races.
Solution: Read once, reset the request body, and pass immutable `[]byte` to workers.
```go
bodyBytes, _ := io.ReadAll(r.Body)
_ = r.Body.Close()
r.Body = io.NopCloser(bytes.NewReader(bodyBytes)) // for primary path if needed

wg := sync.WaitGroup{}
for _, be := range backends {
    wg.Add(1)
    go func(b Backend){
        defer wg.Done()
        req := r.Clone(ctx)
        req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
        doBackendCall(req)
    }(be)
}
wg.Wait()
```
Advantages: deterministic, no atomics, per-call readers isolated.

## Test Logger / Observer Pattern
Test helpers capturing events/logs use:
```go
type MockLogger struct {
    mu  sync.Mutex
    logs []LogEvent
}
func (l *MockLogger) append(e LogEvent){
    l.mu.Lock(); l.logs = append(l.logs, e); l.mu.Unlock()
}
```
Never append to shared slices without a mutex.

## Choosing Between Mutexes, Channels, Atomics
| Scenario | Mutex | Channel | Atomic |
|----------|-------|---------|--------|
| Protect compound invariants (slice + length) | ✅ | ❌ (adds queue semantics) | ❌ |
| Broadcast events to 0..N observers | ✅ (snapshot) | ➖ (requires fan-out goroutines) | ❌ |
| Single-writer, many readers of immutable snapshot | ➖ (still fine) | ❌ | ✅ (atomic pointer swap) |
| Coordinating worker lifecycle / backpressure | ❌ | ✅ | ❌ |
| Reduce memory barrier costs in hot path primitive field | ❌ | ❌ | ✅ (e.g., atomic.Value) |

Guideline: Start with a mutex. Escalate to atomics only with benchmark evidence. Use channels for coordination, not as a lock substitute.

## Common Pitfalls & Anti-Patterns
- Holding a lock while performing network I/O or calling untrusted code.
- Returning internal mutable maps/slices directly (copy before returning if needed).
- Mutating `*http.Request` (URL, Body) across goroutines after dispatch.
- Using channels when a simple mutex suffices (leads to goroutine leaks & harder reasoning).
- Forgetting to close or recreate `r.Body` after a pre-read when handlers downstream still need it.

## Race Detector Integration
All primary CI test jobs (core/unit, modules, BDD, CLI) already run with `-race` and `CGO_ENABLED=1`.

To get immediate local feedback (mirroring CI):
```
GORACE=halt_on_error=1 go test -race ./...
```
Or for a specific module:
```
GORACE=halt_on_error=1 (cd modules/<module> && go test -race ./...)
```
Set `GORACE=halt_on_error=1` to force an immediate test failure on the first detected race (CI sets this automatically in race-enabled steps).
Any race failure must be resolved prior to merging.

## Extending Modules Safely
When adding a new module:
1. Identify mutable shared state; wrap with a struct + mutex.
2. Expose update APIs that replace internal snapshots—never partial in-place mutation across goroutines.
3. If broadcasting, copy observer list under read lock.
4. If parallel fan-out needs request data, pre-buffer.
5. Add unit tests that run under `-race` (invoke with `go test -race ./...`).

## Review Checklist
- Are all shared slices/maps mutated only under lock? (Y/N)
- Are observer notifications done outside lock? (Y/N)
- Any post-construction external map/slice mutation relied upon? (Must be N)
- Any parallel goroutines sharing a request body or mutable struct without cloning? (Must be N)
- Does the module pass `go test -race` in isolation? (Y/N)

## Future Enhancements
- Add optional benchmark-based guidance to decide when atomic snapshot pattern should replace mutex.
- Provide helper utilities for cloning common map types.
- Introduce static analysis lint to flag exported fields of map/slice types.

---
Questions or proposals for deviation should include: rationale, benchmark numbers (if performance-motivated), and race detector run output.
