# Testing Guide

This document explains how to run tests for the cherry-picker project.

## Unit Tests

Run all unit tests:

```bash
go test ./...
```

Run tests with coverage:

```bash
go test -cover ./...
```

Run tests for a specific package:

```bash
go test ./cmd/pick/...
go test ./internal/github/...
```

## Integration Tests

Integration tests use real git operations in temporary repositories. They are tagged with `integration` to keep them separate from unit tests.

Run integration tests:

```bash
go test -tags=integration ./...
```

Run integration tests for a specific package:

```bash
go test -tags=integration ./cmd/pick/...
```

Run integration tests with verbose output:

```bash
go test -tags=integration -v ./cmd/pick/...
```

### What Integration Tests Cover

The integration tests in `cmd/pick/pick_git_integration_test.go` test:

1. **Commit Message Manipulation**: `moveSignedOffByLinesToEnd()` function
   - Reordering Signed-off-by lines to the end of commit messages
   - Handling multiple Signed-off-by lines
   - Preserving commit message body

2. **Git Operations**: Core git helper functions
   - `getCommitInfo()` - Retrieving commit information
   - `getConflictedFiles()` - Detecting merge conflicts
   - `performCherryPick()` - Cherry-picking commits

3. **Real Git Workflows**:
   - Creating temporary git repositories
   - Cherry-picking without conflicts
   - Complex commit message scenarios

### Requirements

Integration tests require:
- Git installed and available in PATH
- No network access required (all tests use local repositories)
- Tests automatically disable GPG commit signing

## Using Makefile Targets

The project includes convenient Makefile targets for running tests:

```bash
# Run unit tests only (fast, default)
make test

# Run integration tests only
make test-integration

# Run all tests (unit + integration)
make test-all

# Run unit tests with coverage report
make test-coverage

# Run all tests with coverage report
make test-coverage-all

# Run all checks (fmt, vet, unit tests)
make check
```

## Coverage Reports

Generate detailed coverage report:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

Current coverage status (unit tests only):

- **High Coverage (>60%)**:
  - internal/config: 84.6%
  - cmd/config: 84.0%
  - cmd/status: 73.4%
  - internal/commands: 65.9%
  - cmd/retry: 64.9%
  - cmd/summary: 64.7%

- **Medium Coverage (30-60%)**:
  - cmd/merge: 57.1%

- **Areas for Improvement (<30%)**:
  - cmd/fetch: 19.9%
  - internal/github: 14.7%
  - cmd/pick: 13.2%

**With integration tests** (`make test-coverage-all`):
- cmd/pick: 13.2% → **37.4%** (+24.2%)

Note: Lower coverage in some packages is expected as they contain GitHub API integration code and git command execution that would require network access or extensive mocking.
