# API Contract Management

This document describes the API contract management functionality provided by the Modular Go framework's `modcli` tool. 

## Scope and Applicability

**The API contract management system is designed to work with any Go codebase**, not just projects using the Modular framework. While it was developed as part of the Modular Go framework, the contract extraction and comparison functionality can be used for:

- **Any Go package or module** - Extract contracts from standard Go libraries, third-party packages, or your own code
- **Go monorepos** - Manage contracts across multiple modules in a single repository  
- **Standalone Go projects** - Track API evolution in single-package projects
- **Library development** - Ensure backward compatibility when publishing Go libraries
- **Enterprise Go codebases** - Maintain API governance across large organizations

The only requirement is that your code follows standard Go package conventions and can be compiled by the Go toolchain.

## Overview

The API contract management system provides:

- **Automated API Contract Extraction**: Extract public API contracts from any Go package
- **Breaking Change Detection**: Identify breaking changes between API versions
- **Git Integration**: Compare contracts across git references (branches, tags, commits)
- **Version Tag Support**: Automatically identify and work with semantic version tags
- **CI/CD Integration**: Automatic contract checking in pull requests
- **Multiple Output Formats**: JSON artifacts, Markdown reports, and plain text summaries

## Quick Start

### Installation

The contract functionality is built into the `modcli` tool:

```bash
cd cmd/modcli
go build -o modcli
```

### Basic Usage

```bash
# Extract API contract from current directory
./modcli contract extract . -o api-contract.json

# Extract contract from a specific module
./modcli contract extract ./modules/auth -o auth-contract.json

# Compare two contract versions
./modcli contract compare v1-contract.json v2-contract.json --format=markdown

# Include private/unexported items
./modcli contract extract . --include-private -v
```

## Commands

### `contract extract`

Extracts the public API contract from a Go package or directory.

```bash
modcli contract extract [package] [flags]

Flags:
  -o, --output string      Output file (default: stdout)
      --include-private    Include unexported items
      --include-tests      Include test files
      --include-internal   Include internal packages
  -v, --verbose            Verbose output
```

**Examples:**
```bash
# Extract from current directory
modcli contract extract .

# Extract from specific module directory
modcli contract extract ./modules/auth

# Extract from remote package
modcli contract extract github.com/GoCodeAlone/modular

# Save to file with verbose output
modcli contract extract . -o contract.json -v
```

### `contract compare`

Compares two API contract files and identifies differences.

```bash
modcli contract compare <old-contract> <new-contract> [flags]

Flags:
  -o, --output string         Output file (default: stdout)
      --format string         Output format: json, markdown, text (default "json")
      --ignore-positions      Ignore source position changes (default true)
      --ignore-comments       Ignore documentation comment changes
  -v, --verbose              Verbose output
```

**Examples:**
```bash
# Compare contracts with JSON output
modcli contract compare old.json new.json

# Generate Markdown report
modcli contract compare old.json new.json --format=markdown -o diff.md

# Compare and save to file
modcli contract compare v1.json v2.json -o changes.json
```

### `contract git-diff`

Compares API contracts between git references (branches, tags, or commits).

```bash
modcli contract git-diff [old-ref] [new-ref] [package-path] [flags]

Flags:
  -o, --output string           Output file (default: stdout)  
      --format string           Output format: json, markdown, text (default "markdown")
      --ignore-positions        Ignore source position changes (default true)
      --ignore-comments         Ignore documentation comment changes
      --baseline string         Baseline reference (for single-ref comparisons)
      --version-pattern string  Pattern for identifying version tags (default "^v\\d+\\.\\d+\\.\\d+.*$")
  -v, --verbose                Verbose output
```

**Examples:**
```bash
# Compare tag v1.0.0 with current working directory
modcli contract git-diff v1.0.0

# Compare two tags
modcli contract git-diff v1.0.0 v1.1.0 

# Compare with specific package path
modcli contract git-diff v1.0.0 main ./modules/auth

# Compare last commit with current state
modcli contract git-diff HEAD~1

# Use baseline flag for cleaner command
modcli contract git-diff --baseline v1.0.0 .
```

### `contract tags`

Lists available version tags that can be used for contract comparison.

```bash
modcli contract tags [package-path] [flags]

Flags:
      --pattern string  Pattern for matching version tags (default "^v\\d+\\.\\d+\\.\\d+.*$")
  -v, --verbose        Show detailed tag information
```

**Examples:**
```bash
# List version tags in current directory
modcli contract tags .

# List with custom pattern
modcli contract tags --pattern "^release-.*"

# Verbose output with dates and commit info
modcli contract tags . -v
```

## Contract Structure

API contracts are JSON documents that capture:

### Interfaces
```json
{
  "name": "AuthService",
  "package": "auth",
  "doc_comment": "AuthService provides authentication functionality",
  "methods": [
    {
      "name": "Login",
      "parameters": [{"name": "username", "type": "string"}],
      "results": [{"type": "error"}]
    }
  ]
}
```

### Types (Structs, Aliases)
```json
{
  "name": "User",
  "package": "auth",
  "kind": "struct",
  "fields": [
    {
      "name": "ID",
      "type": "string",
      "tag": "json:\"id\""
    }
  ]
}
```

### Functions
```json
{
  "name": "NewAuthService", 
  "package": "auth",
  "parameters": [{"name": "config", "type": "*Config"}],
  "results": [{"type": "*AuthService"}]
}
```

### Variables and Constants
```json
{
  "name": "DefaultTimeout",
  "package": "auth", 
  "type": "time.Duration",
  "value": "30s"
}
```

## Change Detection

The system categorizes changes into three types:

### Breaking Changes (🚨)
- Removed interfaces, methods, functions
- Changed method/function signatures
- Removed struct fields
- Changed variable/constant types
- Changed type definitions

### Additions (➕)
- New interfaces, methods, functions
- New struct fields
- New variables and constants
- New types

### Modifications (📝)
- Documentation comment changes
- Struct tag changes
- Constant value changes (non-breaking)

## CI/CD Integration

### GitHub Actions Workflow

The repository includes a GitHub Actions workflow (`.github/workflows/contract-check.yml`) that:

1. **Extracts contracts** from both main branch and PR branch
2. **Compares contracts** for all modules and core framework
3. **Posts PR comments** with contract diff summaries
4. **Fails the build** if breaking changes are detected
5. **Stores artifacts** with full contract diffs

### Workflow Triggers

The workflow runs on:
- Pull requests to `main` branch
- Changes to `**.go`, `go.mod`, or `go.sum` files
- Changes to module `go.mod` files

### Example PR Comment

```markdown
## 📋 API Contract Changes Summary

⚠️ **WARNING: This PR contains breaking API changes!**

### Changed Components:

#### Module: auth

# API Contract Diff: auth

## 🚨 Breaking Changes

### removed_method: AuthService.Login
Method Login was removed from interface AuthService

**Old:**
```go
Login(username string, password string) (bool, error)
```

## ➕ Additions

- **method**: AuthService.LoginWithOAuth - New method LoginWithOAuth was added to interface AuthService
```

## Output Formats

### JSON Format
Structured data suitable for programmatic processing and artifact storage.

### Markdown Format
Human-readable reports perfect for PR comments and documentation.

### Text Format
Simple text output for terminal display and logging.

## Configuration

### Include Options

- **`--include-private`**: Include unexported (private) items in the contract
- **`--include-tests`**: Include test files (`*_test.go`) in extraction
- **`--include-internal`**: Include internal packages in extraction

### Diff Options

- **`--ignore-positions`**: Ignore source file position changes (default: true)
- **`--ignore-comments`**: Ignore documentation comment changes
- **`--format`**: Output format (json, markdown, text)

## Best Practices

### 1. Version Management
```bash
# Tag contracts with versions
modcli contract extract . -o contracts/v1.0.0.json

# Compare against previous version
modcli contract compare contracts/v1.0.0.json contracts/v1.1.0.json
```

### 2. Module-Specific Contracts
```bash
# Extract contracts for each module separately
for module in modules/*/; do
  module_name=$(basename "$module")
  modcli contract extract "$module" -o "contracts/${module_name}.json"
done
```

### 3. Git-Based Workflows
```bash
# Compare current changes with latest release
modcli contract git-diff $(modcli contract tags . | head -n1)

# Compare two releases to generate release notes
modcli contract git-diff v1.0.0 v1.1.0 --format=markdown > release-notes.md

# Check what's changed since last major version
modcli contract git-diff --baseline v1.0.0 --format=markdown

# Pre-commit hook: compare with main branch
modcli contract git-diff origin/main HEAD .
```

### 4. Version Tag Management
```bash
# List available version tags
modcli contract tags .

# Find latest version automatically
latest=$(modcli contract tags . | head -n1)
modcli contract git-diff $latest

# Custom version patterns for different projects
modcli contract tags --pattern "^release-\d+\.\d+$"
```

### 5. Automated Documentation
```bash
# Generate API documentation from contracts
modcli contract compare old.json new.json --format=markdown > CHANGELOG.md
```

### 6. Breaking Change Workflow
1. **Pre-merge**: CI automatically detects breaking changes
2. **Review**: Team reviews breaking changes in PR comments
3. **Decision**: Approve for major version or request changes
4. **Documentation**: Update migration guides and changelogs
5. **Pattern Evaluation**: For each detected breaking or additive change altering existing interfaces/constructors, confirm Builder/Observer alternative was considered & documented (Constitution Art. XII & XVI)

### 7. Pattern-Guided API Evolution
When adding capabilities, first attempt one of:
1. Builder option (additive, backward compatible, default preserves behavior)
2. Observer event (informational or side-effect decoupled)
3. New narrow interface (opt-in) instead of widening an existing one

If none suffice, prepare:
- Deprecation notice in old interface (comment)
- Adapter bridging old to new
- Migration notes referencing decision rationale
- Contract diff attached to PR proving controlled change

Contract review checklist additions:
- [ ] Builder/Observer alternative documented
- [ ] Deprecation + adapter (if interface change)
- [ ] Event schema (if observer) tested & version-stable
- [ ] Defaults of new builder options maintain backward behavior

## Examples

### Extract Core Framework Contract
```bash
modcli contract extract . -v -o core-framework.json
```

Output:
```
Extracting API contract from: .
Saving contract to: core-framework.json
API contract extracted and saved to core-framework.json
Contract extracted successfully:
  - Package: modular
  - Interfaces: 43
  - Types: 33
  - Functions: 18
  - Variables: 65
  - Constants: 14
```

### Compare Module Versions
```bash
modcli contract compare auth-v1.json auth-v2.json --format=markdown
```

Output shows breaking changes, additions, and modifications in a clear format.

## Troubleshooting

### Common Issues

1. **Package not found**: Ensure the package path is correct and the package compiles
2. **Flag conflicts in tests**: Use separate command instances to avoid flag redefinition
3. **Empty contracts**: Check that the package contains exported items
4. **CI failures**: Verify that both old and new contracts are generated successfully

### Debug Options

Use `-v/--verbose` flag for detailed extraction information:

```bash
modcli contract extract . -v
```

This provides insights into:
- Package loading process
- Number of items extracted
- Extraction warnings or errors

## Contributing

When contributing to the API contract functionality:

1. **Run tests**: `go test ./cmd/modcli/internal/contract -v`
2. **Test CLI commands**: Manually test extraction and comparison
3. **Update documentation**: Keep this README current with new features
4. **Consider breaking changes**: API changes to the contract format may require version migration