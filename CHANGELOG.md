# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

This repository is a multi-module Go module; each sub-module under `modules/`
has its own versioning tag (e.g. `modules/auth/v1.16.0`). The core `modular`
package follows its own `vN.M.P` tag stream.

## [Unreleased]
### Added
- Initial CHANGELOG + CONTRIBUTING (QoL sweep follow-up).

## Recent core releases

See `git tag --list 'v*' --sort=-version:refname | head` for the full list.

## Recent sub-module releases

See `git tag --list 'modules/*/v*' --sort=-version:refname | head -20` for
per-module tag history.
