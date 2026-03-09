# BDD/Contract Testing Framework — Reimplementation Plan

> Previously implemented in GoCodeAlone/modular (v1.4.3). Dropped during reset to GoCodeAlone/modular upstream.
> This document captures the design for future reimplementation.

## Overview

The BDD/Contract Testing framework uses Cucumber/Godog for behavior-driven development with Gherkin feature files and Go step definitions. It defines formal contracts for the reload and health subsystems, establishes performance baselines, and enforces a TDD discipline (RED-GREEN-REFACTOR) across a 58-task, 6-phase implementation structure. It also includes API contract management tooling for breaking change detection.

## Key Interfaces

```go
// Contract verification — modules assert compliance with behavioral contracts
type ContractVerifier interface {
    VerifyReloadContract(module Reloadable) []ContractViolation
    VerifyHealthContract(provider HealthProvider) []ContractViolation
}

type ContractViolation struct {
    Contract    string // e.g., "reload", "health"
    Rule        string // e.g., "must-emit-started-event"
    Description string
    Severity    string // "error", "warning"
}

// Contract extraction for API versioning
type ContractExtractor interface {
    Extract(version string) ContractSnapshot
    Compare(old, new ContractSnapshot) []BreakingChange
}

type ContractSnapshot struct {
    Version    string
    Interfaces map[string]InterfaceContract
    Events     []string
    Timestamp  time.Time
}

type BreakingChange struct {
    Type        string // "interface-widened", "method-removed", "signature-changed"
    Interface   string
    Method      string
    Description string
}
```

## Architecture

**Gherkin feature files** cover core framework behaviors:
- `application_lifecycle.feature` — startup, shutdown, signal handling
- `configuration_management.feature` — config loading, validation, env overrides
- `cycle_detection.feature` — module dependency cycle detection and reporting
- `logger_decorator.feature` — structured logging decoration
- `service_registry.feature` — service registration, lookup, type safety
- `base_config.feature` — default config, merging, precedence

**Contract specifications** define formal behavioral requirements:

*Reload contract*:
- Modules implementing `Reloadable` must handle `Reload()` idempotently
- `CanReload()` must be safe to call concurrently and return deterministically
- `ReloadTimeout()` must return a positive duration
- Events must fire in order: Started -> (Completed | Failed)
- On failure, previously applied modules must be rolled back
- Constraint: reload must not block longer than the sum of all module timeouts

*Health contract*:
- `HealthCheck()` must return within a reasonable timeout (default 5s)
- Reports must have non-empty Module and Component fields
- JSON schema validation for health response format
- Aggregation: worst-of for readiness (non-optional), worst-of for health (all)
- Events: `HealthEvaluatedEvent` after every check, `HealthStatusChangedEvent` on transitions only

**Design briefs** (FR-045 and FR-048) provide detailed functional requirements:
- FR-045 (Dynamic Reload): atomic semantics, circuit breaker, event lifecycle, rollback behavior
- FR-048 (Aggregate Health): provider pattern, caching, concurrent collection, panic recovery

**Task structure** — 58 tasks across 6 phases:
1. **Setup** (tasks 1-8): project scaffolding, Godog integration, build tags for pending tests
2. **Tests First** (tasks 9-20): write failing Gherkin scenarios and step definitions
3. **Core Implementation** (tasks 21-35): implement to make tests pass (RED -> GREEN)
4. **Integration** (tasks 36-44): cross-module integration tests, event flow verification
5. **Hardening** (tasks 45-52): performance benchmarks, concurrency stress tests, edge cases
6. **Finalization** (tasks 53-58): documentation, contract extraction tooling, CI integration

**Performance targets**:
- Bootstrap: <150ms P50 with 10 modules
- Service lookup: <2us
- Reload: <80ms P50
- Health aggregation: <5ms P50

**Constitution rules** (non-negotiable design constraints):
- No interface widening — existing interfaces are frozen after v1.0
- Additive only — new functionality via new interfaces or builder options
- Builder options preferred over config struct changes

**API contract management** via `modcli`:
- `modcli contract extract` — snapshot current interfaces, events, types
- `modcli contract compare v1 v2` — detect breaking changes between versions
- CI integration: fail build on breaking changes in non-major version bumps

## Implementation Checklist

- [ ] Add `github.com/cucumber/godog` dependency
- [ ] Create `features/` directory with Gherkin feature files (6 files listed above)
- [ ] Write Go step definitions for application lifecycle scenarios
- [ ] Write Go step definitions for configuration management scenarios
- [ ] Write Go step definitions for cycle detection scenarios
- [ ] Write Go step definitions for service registry scenarios
- [ ] Define reload contract spec as testable assertions
- [ ] Define health contract spec as testable assertions
- [ ] Implement `ContractVerifier` for reload and health contracts
- [ ] Write FR-045 (dynamic reload) Gherkin scenarios before implementation
- [ ] Write FR-048 (aggregate health) Gherkin scenarios before implementation
- [ ] Set up build tags (`//go:build pending`) for tests written before implementation exists
- [ ] Implement core features to pass tests (GREEN phase)
- [ ] Refactor for clarity and performance (REFACTOR phase)
- [ ] Write performance benchmarks for all 4 targets (bootstrap, lookup, reload, health)
- [ ] Write concurrency stress tests (parallel reloads, concurrent health checks, registration races)
- [ ] Implement `ContractExtractor` and `ContractSnapshot` types
- [ ] Implement `modcli contract extract` command
- [ ] Implement `modcli contract compare` command with breaking change detection
- [ ] Add CI step: contract comparison on PRs targeting main

## Notes

- Use `//go:build pending` to keep failing tests compiling but excluded from default `go test` runs until implementation catches up.
- The 58-task structure is a guide, not rigid. Tasks can be parallelized within phases but phases should be sequential.
- Performance targets are P50 values measured on commodity hardware. CI benchmarks should track regressions, not enforce absolute thresholds.
- Constitution rules exist to maintain backward compatibility. Breaking changes require a major version bump and must be flagged by contract tooling.
- Godog integrates with `testing.T` via `godog.TestSuite` — no separate test runner needed.
- Feature files should be human-readable enough for non-engineers to review behavioral expectations.
