# Go Best Practices & Maintenance Guide

Complementary to `memory/constitution.md` (Articles XI–XV). This file provides actionable checklists and examples.

## 1. Interfaces
- Define in consumer package when you need to decouple or mock.
- Avoid exporting interface just to satisfy a single implementation.
- Prefer concrete types internally; wrap only when crossing a boundary.

## 2. Constructors & Options
```go
// Good: functional options keep signature short
func NewClient(opts ...Option) (*Client, error) {}

// Bad: too many primitives
func NewClient(host string, port int, timeout int, retries int, secure bool, logger *slog.Logger) (*Client, error) {}
```
Refactor when >5 positional primitives.

### Builder / Functional Options Guidance (Articles XII & XVI)
- Add capabilities via new option functions or fluent builder methods; NEVER add a required positional param to an existing exported constructor unless a deprecation + adapter path is provided.
- Option naming: `WithX`, where X is a domain term (e.g., `WithRetryPolicy`, not `WithRetriesCfg`).
- Defaults MUST preserve previous behavior (zero-change upgrade). Document default in option comment.
- Side effects deferred until final `Build()` / module `Start` to keep construction deterministic & testable.
- Validate options collectively; return aggregated error list when multiple invalid options supplied.

## 3. Zero-Cost Defaults
Ensure `var m ModuleType` or `&ModuleType{}` is valid to configure minimally.
Provide `DefaultConfig()` when non-zero values required.

## 4. Generics
Use when:
- Eliminating ≥2 near-identical implementations
- Complexity < clarity cost, benchmark shows no regression
Document with a short usage example.

## 5. Reflection
Allowed only in:
- Config feeding / provenance
- Generic helper utilities (one-time path)
Forbidden in hot code paths (service lookup, request handling) unless benchmarked.
Add comment: `// reflection justified: <reason>`

## 6. Error Conventions
- Wrap with context using `%w`
- Sentinel errors in `errors.go`
- Message format: `area: description`
Example: `config: missing database dsn`

## 7. Logging Fields
Common structured keys: `module`, `tenant`, `instance`, `phase`, `event`.
Avoid dynamic key names (cardinality explosion).

## 8. Concurrency
Every mutex or goroutine block gets comment:
```go
// protects: cache map; invariant: entries immutable after set
mu sync.RWMutex
```
Use context cancellation for shutdown; no leaking goroutines.

## 9. Public API Review Checklist
Before exporting a new symbol:
- [ ] Necessary for external user (cannot accomplish via existing API)
- [ ] Stable naming (noun/action consistent with peers)
- [ ] Added to docs and example if user-facing
- [ ] Covered by test exercising real use path
- [ ] Added to API contract tooling (if applicable)

Deprecation pattern:
```go
// Deprecated: use NewXWithOptions. Scheduled removal in v1.9.
```

### Interface Widening Avoidance
- Adding a method to an existing interface forces all implementations to change (breaking). Prefer:
   1. New narrow interface (e.g., `FooExporter`) and type assertion where needed.
   2. Observer event to publish additional info.
   3. Builder option injecting collaborator that adds behavior externally.
- If unavoidable: mark old interface deprecated, provide adapter bridging old to new, document migration steps.

### Observer Pattern Usage
- Emit events for cross-cutting concerns (metrics, auditing, lifecycle, config provenance) instead of adding methods.
- Event contract: name (`domain.action`), payload struct with stable fields, timing (pre/post), error handling (never panic; return error or log).
- Tests MUST assert emission ordering & payload integrity.

### Decision Record Template (commit or PR description)
```
Pattern Evaluation:
Desired change: <summary>
Builder option feasible? <yes/no + rationale>
Observer event feasible? <yes/no + rationale>
Interface change required? <yes/no + justification>
Chosen path: <builder|observer|new interface|interface change with deprecation>
Migration impact: <none|steps>
```

## 10. Boilerplate Reduction
Track repeated snippet occurrences:
- Create helper after ≥3 duplications OR justify in PR why not.
- Candidates: config validation patterns, service registration wrappers, test harness setup.

## 11. Documentation Freshness
Each PR touching code must answer in description:
- Does this add/remove public symbols? If yes, docs updated.
- New config fields? Added tags: `default`, `required`, `desc`.
- Examples changed? Run example tests locally.

## 12. Examples Health
All examples must:
- Build with `go build ./...`
- Pass `go test` if they include tests
- Avoid copying large code blocks from core; import instead

## 13. Performance Guardrails

### When to Add Benchmarks
Add / update a benchmark when you:
- Introduce reflection inside a loop
- Modify service registry lookup or registration logic
- Change synchronization (locks/atomics) on a hot path
- Add allocation-heavy generics
- Modify configuration loading or validation logic
- Change module lifecycle or dependency resolution

### Performance Validation Steps
1. **Baseline Measurement**: Run `go test -bench=. -benchmem` before changes
2. **Post-Change Measurement**: Run benchmarks after implementation
3. **Threshold Analysis**: Flag changes with >10% regression in:
   - ns/op (nanoseconds per operation)
   - allocs/op (allocations per operation)
   - B/op (bytes allocated per operation)
4. **Documentation**: Include benchmark summary in PR if thresholds exceeded

### Service Registry Performance Requirements
The service registry must maintain O(1) lookup performance:
- **Registration**: <1000ns per service for up to 1000 services
- **Name Resolution**: <100ns per lookup with pre-sized maps
- **Interface Resolution**: <500ns per lookup with type caching
- **Memory**: <50 bytes overhead per registered service

### Hot Path Optimization Guidelines
1. **Map Pre-sizing**: Use `ExpectedServiceCount` in RegistryConfig for optimal map capacity
2. **Interface Caching**: Cache reflect.Type lookups to avoid repeated reflection
3. **Lock Granularity**: Prefer RWMutex over Mutex for read-heavy operations
4. **Memory Pools**: Use sync.Pool for frequently allocated objects in hot paths

### Benchmark Execution
```bash
# Run all benchmarks with memory statistics
go test -bench=. -benchmem ./...

# Run service registry benchmarks specifically
go test -bench=Registry -benchmem ./registry

# Compare before/after with benchstat
go test -bench=. -count=5 -benchmem > old.txt
# ... make changes ...
go test -bench=. -count=5 -benchmem > new.txt
benchstat old.txt new.txt
```

### Performance Regression Policy
- **<5% regression**: Generally acceptable for correctness/feature improvements
- **5-10% regression**: Requires justification and follow-up optimization issue
- **>10% regression**: Must be explicitly approved or implementation redesigned

## 14. Panics Policy
Only for programmer errors (impossible states). Document with `// invariant:` comment.
User or config errors return wrapped errors.

## 15. Commit Hygiene
- Squash fixups before merge
- Commit order: failing test -> implementation -> refactor
- No `WIP` commits in main history

## 16. Tooling
Automate where possible:

## 17. Example Module Size Target
New minimal functional module should be ≤75 LOC (excluding tests). If exceeded, add a note: `// NOTE: size justification: <reason>`.

Service registry benchmark harness lives in `service_registry_benchmark_test.go` and is the canonical reference for scale testing registration & lookup performance. Extend (not replace) when new patterns emerge.

### Benchmark Governance
- When modifying core lookup/registration logic, run `go test -bench=Registry -benchmem` locally and include a summary in the PR description if deltas exceed ±10% on ns/op or allocs/op for any scale.
- If intentional performance regressions are introduced (e.g., for correctness or features), justify explicitly and open a follow‑up issue to explore mitigation.

## Lint Configuration Policy

- A single authoritative config: `.golangci.yml` at the repo root. Removed duplicate `.golangci.github.yml` to avoid drift.
- Add new linters only after: (1) zero false positives on current code, (2) documented rationale in this file, (3) PR includes fixes + enforcement.
- If a linter becomes noisy or blocks progress with low value, open a governance issue citing examples before disabling.
---
Maintainers revisit this guide quarterly; propose updates via PR referencing constitution article alignment.

## 18. DDD Boundaries & Glossary
- Each module defines its bounded context; expose only stable service interfaces—keep aggregates/entities internal.
- Maintain a glossary (central or module README) to align ubiquitous language across config fields, logs, and exported symbols.
- Anti-corruption layer wraps external clients; never leak third-party DTOs beyond infrastructure boundary—translate to domain structs.
- Domain services stay pure (no logging/IO); adapters handle side effects.

## 19. Builder & Observer Testing Patterns
Example builder option test skeleton:
```go
func TestClient_WithRetryPolicy(t *testing.T) {
   t.Run("default behavior unchanged", func(t *testing.T) {
      c1, _ := NewClient()
      c2, _ := NewClient(WithRetryPolicy(DefaultRetryPolicy()))
      // assert baseline equality for unaffected metrics / settings
   })
   t.Run("custom policy applied", func(t *testing.T) {
      var called int
      p := RetryPolicy{MaxAttempts: 3}
      c, _ := NewClient(WithRetryPolicy(p))
      _ = c.Do(func() error { called++; return errors.New("x") })
      if called != 3 { t.Fatalf("expected 3 attempts, got %d", called) }
   })
}
```
Observer emission test skeleton:
```go
func TestScheduler_EmitsJobEvents(t *testing.T) {
   var events []JobEvent
   obs := ObserverFunc(func(e Event) { if je, ok := e.(JobEvent); ok { events = append(events, je) } })
   s := NewScheduler(WithObserver(obs))
   s.Schedule(Job{ID: "a"})
   s.RunOnce(context.Background())
   require.Len(t, events, 2) // job.start, job.complete
   require.Equal(t, "a", events[0].ID)
   require.Equal(t, "job.start", events[0].Name)
}
```

## 20. Quick Reference: Pattern Selection
| Goal | Prefer | Avoid |
|------|--------|-------|
| Add optional config | Builder option | New required constructor param |
| Emit cross-cutting info | Observer event | Interface method just returning data |
| Add behavior for subset of consumers | New narrow interface | Widen core interface |
| Extend lifecycle hooks | Observer event | Hard-coded callbacks |
| Provide alternate algorithm | Builder strategy option | Boolean flag explosion |

Document selection in PR using decision record template.
