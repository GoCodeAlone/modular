# Phase 0 Research – Baseline Modular Framework

## Overview
This research consolidates foundational decisions for the Modular framework baseline feature. The objective is to validate feasibility, surface risks, and record rationale before design artifacts.

## Key Decisions

### D1: Module Lifecycle Orchestration
- Decision: Central `Application` orchestrates deterministic start/stop with reverse-order shutdown.
- Rationale: Predictability simplifies debugging and safe resource release.
- Alternatives: Ad-hoc module `Init()` calls in user code (rejected: fragile ordering), event-driven implicit activation (rejected: hidden coupling).

### D2: Dependency Resolution
- Decision: Service registry supporting name-based and interface-based lookup with ambiguity diagnostics + tie-break rules.
- Rationale: Flexibility for polymorphism; reduces manual wiring.
- Alternatives: Only name-based (less flexible), compile-time code generation (higher complexity upfront).

### D3: Configuration Aggregation & Provenance
- Decision: Layered feeders (env, file, programmatic) with field-level provenance and defaults/required validation.
- Rationale: Auditable and reproducible environment setup; essential for compliance.
- Alternatives: Single source config (insufficient real-world flexibility), precedence via implicit order (non-transparent).

### D4: Multi-Tenancy Isolation
- Decision: Explicit tenant context object; per-tenant service scoping + namespace separation.
- Rationale: Clear boundary prevents cross-tenant leakage.
- Alternatives: Global maps keyed by tenant ID (higher accidental misuse risk), separate processes (heavier resource cost for baseline).

### D5: Dynamic Configuration
- Decision: Only fields tagged as dynamic are hot-reloadable via `Reloadable` contract and re-validation.
- Rationale: Minimizes instability; clear contract for runtime mutability.
- Alternatives: Full dynamic reload (risk of inconsistent state), no runtime changes (reduces operational flexibility).

### D6: Error Taxonomy
- Decision: Standard categories (Config, Validation, Dependency, Lifecycle, Security) with wrapping.
- Rationale: Faster triage and structured observability.
- Alternatives: Free-form errors (inconsistent), custom per-module types only (lacks cross-cutting analytics).

### D7: Health & Readiness Signals
- Decision: Per-module status: healthy|degraded|unhealthy with aggregated worst-status health and readiness excluding optional modules.
- Rationale: Operational clarity; supports orchestration systems.
- Alternatives: Binary ready flag (insufficient nuance), custom module-defined semantic (inconsistent UX).

### D8: Scheduling Catch-Up Policy
- Decision: Default skip missed runs; optional bounded backfill (<=10 executions or 1h) configurable.
- Rationale: Prevents resource storms after downtime; preserves operator control.
- Alternatives: Always backfill (risk spike), never allow backfill (lacks business flexibility).

### D9: Certificate Renewal
- Decision: Renew 30 days before expiry, escalate if <7 days remain without success.
- Rationale: Industry best practice buffer; error observability.
- Alternatives: Last-minute renewal (risk outage), fixed shorter window (less resilience to transient CA issues).

### D10: Auth Mechanisms Baseline
- Decision: JWT (HS256/RS256), OIDC Auth Code, API Key, extensible hooks.
- Rationale: Covers majority of backend integration scenarios.
- Alternatives: Custom-only (onboarding burden), add SAML baseline (scope creep for initial baseline).

### D11: Database Engines
- Decision: PostgreSQL primary; MySQL/MariaDB + SQLite test/dev; extensible driver interface.
- Rationale: Balance of capability, portability, local dev convenience.
- Alternatives: Postgres-only (limits adoption), include NoSQL baseline (dilutes initial focus).

### D12: Performance Guardrails
- Decision: Bootstrap <200ms (10 modules), config load <2s (1000 fields), O(1) registry lookup.
- Rationale: Ensures responsiveness for CLI+service startup workflows.
- Alternatives: No targets (risk silent degradation), strict SLAs (premature optimization risk).

### D13: Metrics Cardinality Control
- Decision: Warn when >100 distinct tag values in 10m per metric dimension.
- Rationale: Prevents runaway observability cost.
- Alternatives: Hard cap (may hide signal), no guard (cost/instability risk).

### D14: Versioning & Deprecation Policy
- Decision: SemVer; deprecations announced ≥1 minor release prior; modules declare minimum core version.
- Rationale: Predictable upgrade path.
- Alternatives: Date-based (less dependency clarity), implicit compatibility (risk breakage).

## Risks & Mitigations
| Risk | Impact | Likelihood | Mitigation |
|------|--------|-----------|------------|
| Ambiguous service resolution | Startup failure confusion | Medium | Deterministic tie-break + enumerated diagnostics |
| Unbounded dynamic reload surface | Runtime instability | Low | Opt-in dynamic tagging + re-validation |
| Tenant bleed-through | Data exposure | Low | Mandatory tenant context, scoped registries |
| Scheduler backlog spikes | Resource exhaustion | Medium | Bounded backfill policy |
| Cert renewal persistent failure | TLS outage | Low | Early renewal window + escalation events |
| Observability cost escalation | Cost & noise | Medium | Cardinality warnings |
| Over-dependence on single DB | Portability risk | Low | Multi-engine baseline |
| Interface churn | Upgrade friction | Medium | SemVer + deprecation window |

## Open (Deferred) Considerations
- Extended tracing conventions (span taxonomy) – Phase future.
- Pluggable policy engine for security events.
- Multi-process tenant sharding reference implementation.

## Conclusion
Research complete; no unresolved NEEDS CLARIFICATION items remain. Ready for Phase 1 design.
