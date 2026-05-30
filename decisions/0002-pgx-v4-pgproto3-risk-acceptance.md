# 2. Accept risk: pgx/v4 + pgproto3/v2 vulnerabilities (no upstream fix)

Date: 2026-05-29
Status: Superseded by the fix below (2026-05-29)
Context: GitHub Dependabot alerts #23–#25 (high) and #54–#56 (low)

## Superseded — fixed by removing pgx/v4 entirely (2026-05-29)

The risk-acceptance below is no longer in force: the vulnerable packages have
been **removed from the dependency graph**, not merely accepted.

Root cause: `go-db-credential-refresh@v1.2.1`'s `driver` package imports
`github.com/jackc/pgx/v4/stdlib` unconditionally (for an optional `"pgxv4"`
driver) even though the `database` module only ever uses the `"pgx"` driver,
which the library already maps to **pgx/v5**. So pgx/v4 + pgproto3/v2 were
dead-but-linked.

Fix: forked the library to `github.com/GoCodeAlone/go-db-credential-refresh`
(tag `v1.3.0` + nested `store/awsrds/v1.3.0`), dropping the `"pgxv4"` driver and
its v4 import (pgx/v5, mysql, pq drivers retained). The `database` module + the
`verbose-debug` / `instance-aware-db` examples now import the fork directly
(direct require, not `replace` — `replace` does not propagate to consumers).
`go mod tidy` then drops pgx/v4 + pgproto3/v2 entirely; repo-wide grep confirms
neither remains in any go.mod/go.sum. Build + tests pass.

Note on `exclude`: a go.mod `exclude` of the vulnerable versions does NOT help
on its own — every pgx/v4 (≤4.18.3) and pgproto3/v2 (≤2.3.3) version is
vulnerable, so `exclude` merely forces a *downgrade* to another vulnerable
version while the package is still imported. Removing the importer (the fork) is
what eliminates them; no `exclude` is used.

---

## (Historical) original risk-acceptance

## Context

Dependabot reports two advisories against the `database` module and the
`verbose-debug` / `instance-aware-db` examples:

- **GHSA-jqcq-xjh3-6g23** (HIGH) — Denial of service in
  `github.com/jackc/pgproto3/v2` (vulnerable `<= 2.3.3`).
- **GHSA-j88v-2chj-qfwx** (low) — SQL injection via placeholder confusion with
  dollar-quoted string literals in `github.com/jackc/pgx/v4` (vulnerable
  `<= 4.18.3`).

Both packages are at their final releases in those major lines
(`pgproto3/v2 v2.3.3`, `pgx/v4 v4.18.3`) and **have no patched version** — the
fix lives only in the `pgx/v5` line. Dependabot reports
`first_patched_version: null` for both.

These packages are not imported by this repo's own code. They are pulled in
transitively:

```
modules/database → github.com/davepgreene/go-db-credential-refresh/driver
                 → github.com/jackc/pgx/v4/stdlib → pgx/v4, pgproto3/v2
```

`go-db-credential-refresh` (latest `v1.2.1`) is used for AWS RDS IAM
credential rotation (`iam_store_wrapper.go`, `credential_refresh_store.go` via
`driver.NewConnector`). Its `driver` package imports `pgx/v4/stdlib`
unconditionally and the library has not migrated to `pgx/v5`. The examples
inherit the dependency through the local `replace` of the database module.

## Decision

**Accept the risk and dismiss the six Dependabot alerts as `tolerable_risk`.**
There is no version bump or `replace` that resolves these (no patched `v4`/`v2`
release exists), and dropping the dependency would mean removing or rewriting
the RDS IAM credential-rotation feature against `pgx/v5` — a substantial,
AWS-integration-testable change disproportionate to the assessed exposure.

## Risk assessment

- **pgproto3/v2 DoS (HIGH):** the crash is triggered by a malicious/compromised
  Postgres server sending crafted protocol messages. This deployment connects
  only to trusted AWS RDS endpoints over IAM auth. Not exposed to untrusted
  servers → low real-world risk.
- **pgx/v4 SQLi (low):** requires constructing queries with dollar-quoted
  placeholder confusion. The dependency is used here for connection and
  credential management (IAM token as password) via `database/sql`, not for
  query construction with that pattern → not reachable.

## Follow-up

Revisit when `github.com/davepgreene/go-db-credential-refresh` ships a
`pgx/v5`-based driver (or a maintained fork does). At that point bump the
library, drop `pgx/v4`/`pgproto3/v2`, and re-open/auto-resolve the alerts.
Track upstream: https://github.com/davepgreene/go-db-credential-refresh
