# Contributing to modular

The foundation library for the [GoCodeAlone/workflow](https://github.com/GoCodeAlone/workflow) ecosystem. Used by `workflow`, `ratchet`, `ratchet-cli`, `workflow-cloud`, and many plugins.

## Before contributing

Read [README.md](README.md) for the module lifecycle, dependency model, and observer pattern.

## Local development

```sh
git clone https://github.com/GoCodeAlone/modular.git
cd modular
go build ./...
go test ./...
```

If you have a `go.work` file in a parent directory (multi-repo workspace), use:

```sh
GOWORK=off go build ./...
GOWORK=off go test ./...
```

Sub-modules under `modules/` build independently:

```sh
cd modules/auth
go build ./...
go test ./...
```

## Pull requests

- One feature or bugfix per PR.
- Update CHANGELOG.md with a Keep-a-Changelog entry.
- Add tests covering new behaviour (modular has extensive BDD coverage — match it).
- Run `go vet ./...` before pushing.
- For sub-module changes: bump the sub-module's tag separately from the core package.

## Reporting issues

See the issue templates under `.github/ISSUE_TEMPLATE/`.
