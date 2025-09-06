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
Add / update a benchmark when you:
- Introduce reflection inside a loop
- Modify service registry lookup or registration logic
- Change synchronization (locks/atomics) on a hot path
- Add allocation-heavy generics
Run with: `go test -bench=. -benchmem` inside affected package.

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
