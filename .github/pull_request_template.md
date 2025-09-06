## Summary

Describe the change and its motivation.

## Type of Change
- [ ] Feature
- [ ] Bug fix
- [ ] Documentation
- [ ] Refactor / Tech Debt
- [ ] Performance
- [ ] Build / CI
- [ ] Other

## Checklist (Constitution & Standards)
Refer to `memory/constitution.md` (v1.1.0) and `GO_BEST_PRACTICES.md`.

Quality Gates:
- [ ] Failing test added first (TDD) or rationale provided if test-only change
- [ ] No leftover TODO / WIP / debug prints
- [ ] Lint passes (`golangci-lint`) or documented waiver
- [ ] All tests pass (core, modules, examples, CLI)
- [ ] Performance-sensitive changes benchmarked or noted N/A
- [ ] Public API changes reviewed with API diff (link output or N/A)
- [ ] Deprecations use standard comment format and migration note added

Docs & Examples:
- [ ] Updated `DOCUMENTATION.md` / module README(s) if public behavior changed
- [ ] Examples updated & still build/run (if affected)
- [ ] New/changed config fields have `default`, `required`, `desc` tags
- [ ] Added/updated feature has quickstart or usage snippet (if user-facing)

Go Best Practices:
- [ ] Interfaces only where ≥2 impls or testing seam needed
- [ ] Constructors avoid >5 primitive params (or switched to options)
- [ ] Reflection not used in hot path (or justified comment + benchmark)
- [ ] Concurrency primitives annotated with ownership comment
- [ ] Errors wrapped with context and lowercase messages
- [ ] Logging fields use standard keys (`module`, `tenant`, `instance`, `phase`)

Multi-Tenancy / Instance (if applicable):
- [ ] Tenant isolation preserved (no cross-tenant state leakage)
- [ ] Instance-aware config uses correct prefixes & validated

Security & Observability:
- [ ] No secrets in logs
- [ ] Lifecycle / health / config provenance events emitted where expected

Boilerplate & Size:
- [ ] New minimal module ≤75 LOC or justification provided
- [ ] Duplicate patterns (>2 copies) refactored or rationale provided

Amendments / Governance:
- [ ] If constitution impacted, updated `memory/constitution.md` + checklist

## Testing Notes
Outline test strategy & key scenarios covered.

## Breaking Changes?
Explain impact, deprecation path, migration steps (or mark N/A).

## Screenshots / Logs (optional)
Add any relevant output.

## Additional Notes
Anything else reviewers should know.
