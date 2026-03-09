# BDD/Contract Testing Framework — Revised Implementation Plan

> Reset from CrisisTextLine/modular upstream (2026-03-09). This revision reflects what already exists.

## Gap Analysis

**Already exists (~65%):**
- Godog dependency (`go.mod`: `github.com/cucumber/godog v0.15.1`)
- 21 Gherkin feature files across core + modules
- 121 BDD test files across the codebase
- Core framework BDD: lifecycle, config, cycle detection, service registry, logger decorator
- Module BDD: auth, cache, database, eventbus, httpserver, httpclient, scheduler, reverseproxy, etc.
- Contract CLI: `modcli contract extract|compare|git-diff|tags` (`cmd/modcli/cmd/contract.go`, 636 lines)
- Contract types: `Contract`, `InterfaceContract`, `BreakingChange`, `ContractDiff` (`cmd/modcli/internal/contract/`)
- Contract extractor + differ with tests (1715 lines across 6 files)
- CI: `contract-check.yml` (241 lines) — extracts, compares, comments on PRs
- CI: `bdd-matrix.yml` (215 lines) — parallel module BDD, coverage merging
- BDD scripts: `run-module-bdd-parallel.sh`, `verify-bdd-tests.sh`

**Must implement (depends on Dynamic Reload + Aggregate Health):**
- Reload contract feature file + step definitions (depends on Reloadable interface)
- Health contract feature file + step definitions (depends on HealthProvider interface)
- `ContractVerifier` interface for reload + health contracts
- Performance benchmark BDD (4 targets: bootstrap, lookup, reload, health)
- Concurrency stress test BDD scenarios

## What to Build

Since the BDD infrastructure and contract tooling are fully operational, the remaining work is:

1. **Reload contract BDD** — write after Dynamic Reload is implemented
2. **Health contract BDD** — write after Aggregate Health is implemented
3. **ContractVerifier** — programmatic verification of reload/health behavioral contracts
4. **Performance benchmarks** — formalize the 4 targets as Go benchmarks

## Files

| Action | File | What |
|--------|------|------|
| Create | `features/reload_contract.feature` | Gherkin scenarios for Reloadable contract |
| Create | `features/health_contract.feature` | Gherkin scenarios for HealthProvider contract |
| Create | `reload_contract_bdd_test.go` | Step definitions for reload scenarios |
| Create | `health_contract_bdd_test.go` | Step definitions for health scenarios |
| Create | `contract_verifier.go` | ContractVerifier interface + implementations |
| Create | `contract_verifier_test.go` | Verifier tests |
| Create | `benchmark_test.go` | Performance benchmarks for 4 targets |

## Implementation Checklist

- [x] ~~Add godog dependency~~ (exists)
- [x] ~~Create features/ directory with core Gherkin files~~ (6 files exist)
- [x] ~~Write step definitions for lifecycle, config, cycle detection, service registry~~ (121 BDD tests)
- [x] ~~Implement ContractExtractor and ContractSnapshot~~ (contract package complete)
- [x] ~~Implement modcli contract extract/compare~~ (636-line CLI)
- [x] ~~Add CI contract comparison on PRs~~ (contract-check.yml)
- [ ] Create reload_contract.feature (after Dynamic Reload is implemented)
- [ ] Write reload contract step definitions
- [ ] Create health_contract.feature (after Aggregate Health is implemented)
- [ ] Write health contract step definitions
- [ ] Implement ContractVerifier for reload contracts
- [ ] Implement ContractVerifier for health contracts
- [ ] Write performance benchmarks (bootstrap <150ms, lookup <2us, reload <80ms, health <5ms)
- [ ] Write concurrency stress test scenarios

## Performance Targets

| Metric | Target (P50) |
|--------|-------------|
| Bootstrap (10 modules) | <150ms |
| Service lookup | <2us |
| Reload | <80ms |
| Health aggregation | <5ms |

## Notes

- Reload/health contract BDD depends on those features being implemented first.
- Performance targets are P50 on commodity hardware; CI tracks regressions, not absolutes.
- Constitution rules (no interface widening, additive only) are already enforced by contract-check.yml.
- Godog integrates with `testing.T` via `godog.TestSuite`.
- Feature files should be readable by non-engineers.
