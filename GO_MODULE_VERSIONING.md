# Go Module Versioning Guide

This document explains how the Modular framework handles Go semantic versioning for the core framework and its modules.

## Overview

Go modules follow semantic versioning (semver) with a special requirement for major versions 2 and above: the module path in `go.mod` must include a version suffix (e.g., `/v2`, `/v3`).

## Version Naming Rules

### For v0.x.x and v1.x.x

**No module path suffix is required.**

```go
// go.mod
module github.com/GoCodeAlone/modular
```

Tags:
- Core: `v1.0.0`, `v1.1.0`, `v1.2.3`
- Modules: `modules/reverseproxy/v1.0.0`, `modules/auth/v1.2.3`

### For v2.x.x and Higher

**Module path MUST include the `/vN` suffix.**

```go
// go.mod for v2
module github.com/GoCodeAlone/modular/v2
```

Tags:
- Core: `v2.0.0`, `v2.1.0`, `v3.0.0`
- Modules: `modules/reverseproxy/v2.0.0`, `modules/auth/v3.1.0`

## Automated Workflow Behavior

The release workflows automatically handle major version transitions using a two-step process:

### When Releasing v2.0.0 or Higher

The workflow uses a **two-step PR-based approach** to ensure safe major version transitions:

#### Step 1: Module Path Update (Automated)

1. **Version Determination**: The workflow calculates the next version based on contract changes and user input
2. **Module Path Validation**: Before creating a release:
   - Checks if releasing v2+ version
   - Validates current module path in `go.mod`
   - If no version suffix exists, the workflow will:
     - Create a new branch with the module path update
     - Update `go.mod` to add the `/vN` suffix
     - Run `go mod tidy` to ensure consistency
     - Create a PR with the changes
     - **Pause the release workflow**
   - If version suffix exists but doesn't match, fails with error
   - If version suffix is already correct, proceeds to Step 2

#### Step 2: Complete the Release (Manual Trigger)

After the module path update PR is merged:

1. **Re-run the Release Workflow**: Trigger the same release workflow again with the same parameters
2. **Module Path Check**: The workflow detects the module path is now correct
3. **Release Creation**: Creates the GitHub release with the correct tag
4. **Go Proxy Announcement**: Announces to Go proxy using the correct module path

### Why Two Steps?

This two-step approach provides several benefits:
- **Safer Process**: The module path change is reviewed in a separate PR before the release
- **Cleaner History**: The module path update is a distinct commit that can be traced
- **Better Practices**: Follows GitFlow and PR review practices
- **Rollback Safety**: The module path update can be reviewed and adjusted before the release is finalized

### Example Workflow

**Initial State (v1.x.x):**
```go
// modules/reverseproxy/go.mod
module github.com/GoCodeAlone/modular/modules/reverseproxy
```

**Step 1: Trigger v2.0.0 Release**

When you trigger a release for v2.0.0, the workflow:
1. Detects that next version is v2.0.0
2. Checks if module path needs updating
3. Creates a new branch: `chore/reverseproxy-update-module-path-v2`
4. Updates `go.mod` module path to include `/v2`
5. Runs `go mod tidy`
6. Commits the changes
7. Creates a PR with the title: "chore(reverseproxy): update module path for v2"
8. **Pauses with message**: "A PR has been created... Please merge the PR, then re-run this release workflow"

**Step 2: Complete the Release**

After merging the PR:
```go
// modules/reverseproxy/go.mod (now updated)
module github.com/GoCodeAlone/modular/modules/reverseproxy/v2
```

Re-run the same release workflow, and it will:
1. Detect module path is already correct for v2
2. Skip the PR creation
3. Create tag `modules/reverseproxy/v2.0.0`
4. Generate release with changelog
5. Announce `github.com/GoCodeAlone/modular/modules/reverseproxy/v2@v2.0.0` to Go proxy

## Manual Version Updates

If you need to manually prepare for a v2+ release:

### Core Framework

```bash
# 1. Update go.mod
sed -i 's|^module github.com/GoCodeAlone/modular$|module github.com/GoCodeAlone/modular/v2|' go.mod

# 2. Update import paths in all .go files (if any self-imports)
find . -name "*.go" -type f -not -path "*/modules/*" -not -path "*/examples/*" \
  -exec sed -i 's|github.com/GoCodeAlone/modular"|github.com/GoCodeAlone/modular/v2"|g' {} +

# 3. Run go mod tidy
go mod tidy

# 4. Test
go test ./...
```

### Module

```bash
MODULE_NAME="reverseproxy"  # Change this to your module name
MAJOR_VERSION="2"           # Change to your target major version

# 1. Update go.mod
sed -i "s|^module github.com/GoCodeAlone/modular/modules/${MODULE_NAME}$|module github.com/GoCodeAlone/modular/modules/${MODULE_NAME}/v${MAJOR_VERSION}|" \
  modules/${MODULE_NAME}/go.mod

# 2. Update import paths (if module has self-imports - rare)
find modules/${MODULE_NAME} -name "*.go" -type f \
  -exec sed -i "s|github.com/GoCodeAlone/modular/modules/${MODULE_NAME}\"|github.com/GoCodeAlone/modular/modules/${MODULE_NAME}/v${MAJOR_VERSION}\"|g" {} +

# 3. Run go mod tidy
cd modules/${MODULE_NAME}
go mod tidy

# 4. Test
go test ./...
```

## Importing v2+ Modules

When using v2+ versions in your code:

```go
// For v1.x.x
import "github.com/GoCodeAlone/modular/modules/reverseproxy"

// For v2.x.x
import "github.com/GoCodeAlone/modular/modules/reverseproxy/v2"

// For v3.x.x
import "github.com/GoCodeAlone/modular/modules/reverseproxy/v3"
```

In `go.mod`:
```go
require (
    github.com/GoCodeAlone/modular/v2 v2.0.0
    github.com/GoCodeAlone/modular/modules/reverseproxy/v2 v2.0.0
)
```

## Breaking Changes and Major Versions

According to semantic versioning:
- **Breaking changes** require a major version bump (e.g., v1.5.0 → v2.0.0)
- **Backward-compatible additions** require a minor version bump (e.g., v1.5.0 → v1.6.0)
- **Bug fixes** require a patch version bump (e.g., v1.5.0 → v1.5.1)

Our workflows use contract-based detection to suggest appropriate version bumps, but you can override this with manual version input or by selecting a different release type.

## Two-Step Release Process for v2+

When releasing a major version v2 or higher for the first time, follow this process:

### For Core Framework

1. **Trigger the Release Workflow**:
   - Go to Actions → Release workflow
   - Select release type "major" or specify version "v2.0.0"
   - Click "Run workflow"

2. **Wait for PR Creation**:
   - The workflow will detect the module path needs updating
   - It will create a PR with the title: "chore: update module path for v2"
   - The workflow will pause and display: "⚠️ RELEASE PAUSED"

3. **Review and Merge the PR**:
   - Review the automatically created PR
   - Ensure tests pass
   - Merge the PR into your base branch

4. **Complete the Release**:
   - Go back to Actions → Release workflow
   - Run the same release workflow again with the same parameters
   - This time, the workflow will detect the module path is correct
   - It will complete the release and create the tag

### For Module Releases

The process is identical, just use the Module Release workflow instead:

1. **Trigger Module Release**: Select your module and choose "major" release type
2. **Wait for PR**: A PR will be created for the module path update
3. **Merge the PR**: Review and merge the module-specific PR
4. **Complete Release**: Re-run the module release workflow

## Troubleshooting

### Error: "RELEASE PAUSED - A PR has been created..."

This is **expected behavior** for first-time v2+ releases. This is not an error; it's the workflow informing you that a PR has been created. Follow the steps above to merge the PR and complete the release.

### Error: "module contains a go.mod file, so module path must match major version"

This error occurs when:
1. You're trying to release v2.0.0 or higher
2. The module path in `go.mod` doesn't include the `/vN` suffix

**Solution**: The workflow creates a PR automatically to fix this. If you see this error during a manual operation, manually update the module path as described in the "Manual Version Updates" section.

### Error: "Module path has /vX but releasing vY"

This error occurs when the module path already has a version suffix, but it doesn't match the version you're trying to release.

**Solution**: Either:
1. Update the version number to match the existing suffix, or
2. Manually update the module path suffix to match your target version

### Downgrading Major Versions

**You cannot downgrade major versions** (e.g., from v3 to v2). If you need to maintain an older major version:
1. Create a branch from the appropriate tag (e.g., `v2-maintenance` from `v2.5.0`)
2. Apply fixes to that branch
3. Release patch versions on that branch (e.g., v2.5.1, v2.5.2)

## References

- [Go Modules: v2 and Beyond](https://go.dev/blog/v2-go-modules)
- [Go Module Reference](https://go.dev/ref/mod)
- [Semantic Versioning](https://semver.org/)

## Testing

To test the version handling logic locally, run:

```bash
./scripts/test-version-handling.sh
```

This demonstrates how versions are mapped to module paths.
