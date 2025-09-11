# /review-prs Command

## Purpose
Automatically review active GitHub Copilot PRs, verify implementation quality, check CI status, and manage PR lifecycle.

## Usage
```
/review-prs
```

## Process
1. **Load Active PRs**: Read from `.claude/pr-tracker.json`
2. **Code Review**: Use go-ddd-code-reviewer agent for each PR
3. **CI Status Check**: Verify all checks pass (tests, linter, etc.)
4. **Goal Verification**: Ensure implementation matches stated objectives
5. **Issue Reporting**: Post review comments tagging @copilot for issues
6. **Auto-Merge**: Merge PRs that meet all criteria
7. **Local Sync**: Pull merged changes locally
8. **Cleanup**: Remove completed PRs from tracker

## Review Criteria
- ✅ All CI checks pass
- ✅ No test failures
- ✅ No linter issues
- ✅ No placeholder/TODO logic
- ✅ Implementation matches PR goals
- ✅ Code follows Go best practices
- ✅ Domain-driven design compliance
- ✅ Proper error handling

## Auto-Review Schedule
- Can be run manually
- Should be triggered after PR creation
- Recommended: every 15 minutes for active PRs

## Actions on Issues
- Post detailed review comment
- Tag @copilot for automated response
- Keep PR in active tracking until resolved