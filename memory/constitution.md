# Modular Framework Project Constitution

**Scope**: Governs design, implementation, testing, and evolution of the Modular framework and bundled modules.

**Version**: 1.2.0 | **Ratified**: 2025-09-06 | **Last Amended**: 2025-09-07

---

## Core Principles

### I. Library-First & Composable
All functionality is delivered as modules or core libraries. No feature is implemented solely inside ad-hoc application code. Each module:
- Clearly states purpose and dependencies
- Implements required framework interfaces directly (no adapter boilerplate unless justified)
- Is independently testable and documented

### II. Deterministic Lifecycle & Dependency Transparency
Module registration, dependency resolution, and start/stop ordering MUST be deterministic and inspectable. Circular dependencies are rejected with explicit cycle reporting. Shutdown strictly reverses successful start order.

### III. Configuration Integrity & Provenance
All configuration derives from declared feeders (environment, file, programmatic). Every field has: source (provenance), default (optional), required flag, description, and optional dynamic flag. Missing required values or invalid data abort startup.

### IV. Multi-Tenant & Instance Isolation
Tenant and instance contexts provide hard isolation boundaries. Cross-tenant leakage (data, cache, services) is prohibited. Tenant-specific services require explicit tenant context.

### V. Observability & Accountability
Lifecycle events, health states, configuration provenance, and error taxonomy (Config, Validation, Dependency, Lifecycle, Security) MUST be emitted or derivable. Metrics cardinality guardrails warn on potential explosion (>100 tag values / 10m per dimension).

### VI. Test-First (NON-NEGOTIABLE)
No production code without a failing test first. Red → Green → Refactor loop is mandatory. Every new feature/update includes:
- Gherkin/BDD scenario(s) mapping to acceptance criteria
- Integration tests exercising real module interactions (not mocks for core behavior)
- Unit tests only for pure logic or boundary transformations

### VII. Realistic Testing & Fidelity
Tests MUST execute actual framework code paths and real integrations where defined (e.g., real Postgres or ephemeral in-memory substitute only when semantically equivalent). Forbidden:
- Mocking core lifecycle or service registry
- Tests asserting only that mocks are called without validating observable outcomes
- Synthetic scenarios that omit error handling or boundary conditions present in spec

### VIII. Extensibility with Restraint
Extension points (decorators, observers, feeders, modules) are provided where concrete needs exist. Speculative abstraction is deferred until at least two distinct use cases demand it.

### IX. Semantic Versioning & Predictable Evolution
Core and modules follow SemVer. Breaking changes require a deprecation path ≥1 minor version and documented migration notes.

### X. Performance & Operational Baselines
- Bootstrap (<200ms for 10 modules typical target)
- Config load (<2s for 1000 fields)
- O(1) expected service lookup
- Bounded scheduler catch-up (default skip, optional limited backfill)

### XI. Idiomatic Go & Boilerplate Minimization
Modules and core packages MUST embrace idiomatic Go:
- Prefer composition over inheritance-like indirection; keep exported surface minimal.
- Avoid unnecessary interface abstraction; define an interface in the consumer package only when ≥2 implementations or a mock need exists.
- Zero-cost defaults: constructing a config or module with zero values should be valid unless explicitly unsafe.
- No reflection in hot paths unless justified with benchmark + comment.
- Cap constructor parameter count via option structs/functional options (>5 primitive params requires refactor).
- Generics: only when they measurably reduce duplication without harming clarity (add benchmark or LOC delta reference in PR description).
- Eliminate duplicate helper utilities by centralizing into internal packages when shared ≥2 call sites.

### XII. Public API Stability & Review
Any exported (non-internal) symbol constitutes public API. Changes gated by:
- API diff tooling (see `API_CONTRACT_MANAGEMENT.md`) must show no unintended removals/semantic changes.
- Adding exported symbols requires rationale & usage example in docs or examples.
- Deprecations use `// Deprecated: <reason>. Removal in vX.Y (≥1 minor ahead).` comment form.
- Removal only after at least one released minor version containing deprecation notice.
 - Additive changes that alter constructor or interface method signatures (even if they compile for existing callers using type inference) are treated as potential breaking changes and MUST first be evaluated for delivery via the Builder pattern (additional fluent option) or Observer pattern (decoupled event/listener) to minimize disruption.
 - Prefer evolving configuration and extensibility surfaces through: (1) new Builder option methods with sensible defaults, (2) optional functional options, or (3) observer hooks, before mutating existing interfaces.
 - Interface widening (adding a method) is forbidden without a deprecation + adapter path; instead, introduce a new narrow interface and have existing types opt-in, or expose capability via an observer or builder-provided service.

### XIII. Documentation & Example Freshness
Documentation is a living contract:
- Every new feature/update: docs + examples updated in same PR; failing to do so blocks merge.
- Root `DOCUMENTATION.md` + module READMEs MUST not reference removed symbols (enforced by periodic doc lint task—future automation placeholder).
- Examples must compile & pass tests; stale examples are treated as defects.
- Config field additions require: description tag, default (if optional), and provenance visibility.
- A “Why it exists” paragraph accompanies any new module or extension point.

### XIV. Boilerplate Reduction Targets
We continually measure and reduce ceremony:
- New minimal module (config + one service) target: ≤75 lines including tests scaffold (excluding generated mocks).
- If repeated snippet appears ≥3 times (excluding tests), refactor or justify in PR.
- Provide code generators (CLI) only after manual pattern stabilizes across ≥2 modules.

### XV. Consistency & Style Enforcement
- `golangci-lint` must pass (or documented waivers with justification + issue link).
- Uniform logging fields: `module`, `tenant`, `instance`, `phase` where applicable.
- Error messages start lowercase, no trailing punctuation, and include context noun first (e.g., `config: missing database host`).
- Panics restricted to programmer errors (never for invalid user config) and documented.
- All concurrency primitives (mutexes, channels) require a brief comment describing ownership & lifecycle.

### XVI. Strategic Patterns (Builder, Observer, Domain-Driven Design)
The project intentionally standardizes on these patterns to enable low-friction evolution and clear domain boundaries:

1. Builder Pattern
	- All complex module/application construction SHOULD expose a builder (or functional options) to allow additive evolution without breaking existing callers.
	- New optional capabilities MUST prefer builder option methods (or functional options) over adding required constructor parameters.
	- Required additions should be extremely rare; if needed, provide a transitional builder option that derives a sensible default while emitting a deprecation notice for future mandatory requirement.
	- Builder options MUST be side-effect free until `.Build()` / finalization is invoked.

2. Observer Pattern
	- Cross-cutting concerns (metrics emission, auditing, tracing, lifecycle notifications) MUST prefer observers instead of embedding new dependencies into existing module interfaces.
	- New event types require: clear naming (`lifecycle.*`, `config.*`, `tenant.*`), documented payload contract, and tests asserting emission timing & ordering.
	- Avoid tight coupling: observers should depend only on stable event contracts, not concrete module internals.

3. Domain-Driven Design (DDD)
	- Modules map to bounded contexts; a module's exported services form its public domain API.
	- Ubiquitous language: configuration field names, log keys, and service method names reflect domain terms consistently.
	- Aggregates enforce invariants internally; external packages manipulate them only through exported behaviors (not by mutating internal state structs).
	- Anti-corruption layers wrap external systems; never leak external DTOs beyond the boundary—translate to domain types.
	- Domain logic remains decoupled from transport (HTTP, CLI, messaging). Adapters live in dedicated subpackages or modules.

4. API Evolution via Patterns
	- Before modifying an existing interface or constructor, authors MUST document (in PR description) why a Builder or Observer extension is insufficient.
	- Event-based (Observer) extension is preferred for purely informational additions; Builder extension is preferred for configuration or capability toggles.
	- When neither pattern suffices and an interface change is unavoidable, provide: (a) deprecation of old interface, (b) adapter implementation bridging old to new, (c) migration notes, (d) versioned removal plan per Article XII.

Compliance with this article is part of API review; reviewers should request justification when direct interface mutation occurs.

---

## Additional Constraints & Standards
- Dynamic configuration limited to fields explicitly tagged `dynamic`; hot reload performs full validation before applying.
- Secrets must never be logged in plaintext; provenance redacts values.
- Scheduling backlog policies are configurable and bounded.
- Certificate renewal begins 30 days pre-expiry; escalation if <7 days remaining without success.
- Health model: healthy|degraded|unhealthy; aggregate readiness excludes optional module failures.

---

## Development Workflow & Quality Gates

### Workflow Phases
1. Specification (spec.md) → clarifications resolved.
2. Planning (plan.md) → research, data model, contracts, quickstart.
3. Task Generation (tasks.md) → ordered test-first tasks.
4. Implementation → execute tasks in TDD order.
5. Validation → run full lint + tests + performance smoke.

### Mandatory Artifacts per Feature
- spec.md with acceptance scenarios
- plan.md with research & contracts references
- research.md (decisions + rationale)
- data-model.md, contracts/*, quickstart.md
- BDD tests covering each acceptance scenario
- tasks.md (pre-implementation)

### Testing Requirements
- Each acceptance scenario → at least one Gherkin scenario (.feature) or structured BDD equivalent.
- Integration tests MUST verify real side-effects (e.g., service registry resolution, lifecycle ordering, config validation failures).
- Edge cases (error paths, invalid config, multi-tenant isolation) MUST have explicit test coverage.
- Prohibited: tests that only assert method call order on mocks; replace with observable state or output assertions.

### Gate Checks (PR MUST show):
- All new/changed required fields documented with description & default (if any)
- No unresolved TODO placeholders in production code
- Lint passes or justified exceptions documented in PR description
- All tests pass across: core, each module, examples, CLI
- Added capability mapped to at least one failing test prior to implementation (reviewers verify commit history ordering where feasible)

### Performance & Observability Checks
- Startup path measured when adding new module categories (baseline vs previous run)
- Cardinality warnings investigated (either reduce dimensions or justify)
- Health and lifecycle event emission verified in integration tests for new modules.

---

## BDD & Gherkin Policy
- Every feature's acceptance criteria must be mirrored in a Gherkin `.feature` file (or equivalent structured test) using Given/When/Then.
- Scenarios must avoid artificial stubs for core flows (e.g., do not stub module Start; invoke real start sequence).
- Scenario failure messages must guide debugging (include module name, phase, expectation).

---

## Governance
- This Constitution supersedes ad-hoc practices. Deviations require an amendment PR updating this file with rationale and migration notes.
- Reviewers enforce: TDD compliance, realistic test fidelity, semantic versioning, deprecation process.
- Complexity must be justified in PR description if exceeding norms (e.g., adding new global coordination mechanism).
- Any new extension point demands: documented concrete use cases, tests, and quickstart update.
- Amendments require consensus (≥2 maintainers approval) and version increment of this document.

---

## Amendment Log
- 1.2.0 (2025-09-07): Added Article XVI emphasizing Builder, Observer, and DDD patterns; strengthened Article XII with guidance on using patterns for API evolution.
- 1.1.0 (2025-09-06): Added Articles XI–XV covering idiomatic Go, API stability, documentation freshness, boilerplate targets, and style enforcement.
- 1.0.0 (2025-09-06): Initial project-specific constitution established.
