# Contributing to modular/reverseproxy

This is a sub-module of [GoCodeAlone/modular](https://github.com/GoCodeAlone/modular) — a separately-versioned Go module under `modules/reverseproxy/`. Its tag stream is `modules/reverseproxy/vN.M.P`, independent of the core `modular` package.

## Before contributing

Read the top-level [CONTRIBUTING.md](../../CONTRIBUTING.md) for general conventions.

## Local development

Sub-module builds standalone:

```sh
cd modules/reverseproxy
go build ./...
go test ./...
```

If you have a `go.work` file in a parent directory (multi-repo workspace):

```sh
GOWORK=off go build ./...
GOWORK=off go test ./...
```

## Pull requests

- One feature or bugfix per PR. Keep changes scoped to this sub-module.
- Update CHANGELOG.md (in this sub-module's directory) with a Keep-a-Changelog entry.
- Add tests covering new behaviour — match this sub-module's existing test style.
- Run `GOWORK=off go vet ./...` before pushing.
- Bump the sub-module tag (`modules/reverseproxy/vN.M.P`) separately from the core `modular` tag.

## Reporting issues

Use the issue templates under `.github/ISSUE_TEMPLATE/` at the repo root.
