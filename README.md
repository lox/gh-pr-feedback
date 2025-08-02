# gh-pr-feedback

GitHub CLI extension that extracts unresolved PR review feedback and failing checks.

## Installation

```bash
# Install from GitHub repository
gh extension install lox/gh-pr-feedback

# Or install from local path
gh extension install .
```

## Usage

```bash
# Current directory
gh pr-feedback

# Specific directory
gh pr-feedback /path/to/repo

# JSON output
gh pr-feedback --json
```

## Output Example

```
! Add certificate management PR #117 · https://github.com/owner/repo/pull/117

REVIEW COMMENTS
! Consider using a more specific error message here
  reviewer: src/main.go#42

FAILED CHECKS
X Shellcheck in 6s (ID 16637926739)

To see what failed, try:
  gh run view 16637926739

Found 1 unresolved comment(s) and 1 failing check(s)
```

## Features

- Detects current PR automatically
- Shows unresolved review comments with file/line locations
- Lists failing status checks with run IDs
- Filters out resolved discussions
- JSON output for automation (`--json`)
