# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This repository contains two CLI tools for managing GitHub PRs:

1. **cherry-picker**: Manages cherry-picks across release branches with AI-assisted conflict resolution
2. **dep-merger**: Manages dependency PRs (those with `type/dependencies` label) with retry and merge operations

Both tools track PR status in YAML configuration files and share common GitHub API infrastructure.

## Build and Test Commands

```bash
# Build both binaries
make build

# Build individual binaries
make cherry-picker
make dep-merger

# Run all checks (format, vet, test)
make check

# Run tests only
make test
# Or with verbose output
go test -v ./...

# Format code
make fmt

# Run go vet
make vet

# Clean build artifacts
make clean
```

## Cherry-Picker Architecture

### Core Components

**main.go**: Entry point using Cobra for CLI commands. All commands have access to a global `--config` flag (default: `cherry-picks.yaml`).

**cmd/config.go**: Defines core data structures:
- `Config`: Repository configuration with org, repo, source branch, AI assistant command, and tracked PRs
- `TrackedPR`: PR tracking with per-branch status
- `BranchStatus`: Status types (pending, failed, picked, merged) and optional PR details
  - `pending`: Bot hasn't attempted cherry-pick yet
  - `failed`: Bot attempted but failed (usually conflicts) - **pick command works on this status**
  - `picked`: Bot successfully created cherry-pick PR - **pick --force can amend these**
  - `merged`: Cherry-pick PR merged
- `PickPR`: Cherry-pick PR details including number, title, and CI status

**internal/config/config.go**: YAML marshaling/unmarshaling for configuration persistence.

**internal/github/client.go**: GitHub API wrapper using `google/go-github/v57`:
- Fetches merged PRs since a date
- Retrieves PR details with CI status (checks both combined status and check runs)
- DCO check filtering (ignores DCO status when determining CI health)
- Creates PRs, merges with squash method, retries failed workflows
- Supports semantic versioning tags and commit comparisons

**internal/commands/**: Common utilities for command implementation (base command struct, validation helpers, etc.)

### Commands (cmd/ directory)

Each command is in its own package with a `New<Command>Cmd()` factory function:

- **config**: Initialize/update configuration (auto-detects from git)
- **fetch**: Fetch PRs with `cherry-pick/*` labels and detect bot-created cherry-pick PRs and failures
  - Extracts branches from labels (e.g., `cherry-pick/3.6` → `release-3.6`)
  - Scans PR comments for bot activity:
    - Success pattern: "Cherry-pick PR created for X.Y: #NNNN"
    - Failure pattern: "cherry-pick.*failed.*for X.Y"
  - Auto-marks status:
    - `pending`: Label exists but no bot action yet
    - `failed`: Bot attempted but failed
    - `picked`: Bot created PR successfully
    - `merged`: Cherry-pick PR merged
- **pick**: AI-assisted cherry-pick for PRs that bots couldn't handle (bot failures)
  - **Normal mode**: Works on PRs with `failed` status (bot attempted but failed)
  - **Force mode** (`--force`): Amends existing bot-created PRs with `picked` status
  - Uses configured AI assistant for interactive conflict resolution or amendments
  - Performs git operations and creates/updates cherry-pick PRs
- **retry**: Retry failed CI workflows via GitHub Actions API
- **merge**: Squash merge PRs with passing CI
- **status**: Display tracked PRs with GitHub API enrichment (CI status, merge status, suggested commands)
- **summary**: Generate release summary with commits since last tag

### Cherry-Pick Flow (AI-Assisted)

The `pick` command (`cmd/pick/pick.go`) is specifically for handling cherry-picks that automated bots couldn't complete. It orchestrates:
1. **Validate PR has `failed` status** (bot attempted but failed)
2. Fetch PR merge commit SHA
3. For each target branch:
   - Checkout and reset to upstream
   - Create cherry-pick branch (`cherry-pick-<prnum>-<target>`)
   - Execute `git cherry-pick -x --signoff`
   - On conflicts: launch interactive AI assistant session with context prompt
   - Post-AI: verify conflicts resolved, complete cherry-pick
   - Reorder Signed-off-by lines to end of commit message
   - Push branch
   - Create PR via GitHub API
   - Save status immediately (incremental saves per branch)

**Workflow Context**: Normal cherry-picks are handled by automated bots (e.g., argo-cd-cherry-pick-bot). The `pick` command is only needed when:
- Bot attempted cherry-pick but **failed** (status: `failed`)
- Failure usually indicates merge conflicts
- Human intervention with AI assistance is required to resolve conflicts
- PRs with `pending` status should wait for bot to attempt first

### Force Amend Flow (`pick --force`)

The `--force` flag enables amending existing bot-created cherry-pick PRs that need manual fixes:
1. **Validate PR has `picked` status** with existing cherry-pick PR number
2. Fetch existing PR branch via `git fetch origin pull/<number>/head:pr-<number>`
3. Checkout the fetched branch
4. Launch AI assistant with amend-specific prompt (different from conflict resolution)
5. Get remote branch name via GitHub API
6. Force push to update the existing PR
7. Save status (CI status reset to `pending` since it will re-run)

**Use cases for `--force`:**
- CI failures on bot-created PR that need code fixes
- Reviewer feedback requiring changes
- Additional modifications needed for the release branch
- Bot created incorrect cherry-pick that needs manual correction

### AI Conflict Resolution

Uses configured AI assistant CLI (cursor-agent, claude, or custom) for interactive conflict resolution:
- **Configuration**: AI assistant command stored in `Config.AIAssistantCommand` (required field)
- Detects merge conflicts (exit code 1)
- Generates detailed context prompt with conflicted files and commit info
- Displays prompt for user to copy/paste into AI assistant
- Launches configured AI assistant in interactive mode
- After session: validates conflicts resolved before continuing

**Supported AI Assistants:**
- `cursor-agent`: Anthropic's Cursor AI agent CLI
- `claude`: Anthropic's Claude CLI
- Custom: Any command-line tool that provides interactive session

## Cherry-Picker Configuration File

`cherry-picks.yaml` schema:
```yaml
org: string
repo: string
source_branch: string
ai_assistant_command: string  # Required: AI assistant CLI command
last_fetch_date: time.Time
tracked_prs:
  - number: int
    title: string
    branches:
      <branch-name>:
        status: pending|failed|picked|merged
        pr:  # Only present for picked/merged status
          number: int
          title: string
          ci_status: passing|failing|pending|unknown
```

---

## Dep-Merger Architecture

**Entry point**: `cmd/dep-merger/main.go` - Standalone binary using Cobra for CLI commands. All commands have access to a global `--config` flag (default: `dep-merger.yaml`).

### Key Differences from Cherry-Picker

| Aspect | Cherry-Picker | Dep-Merger |
|--------|---------------|------------|
| Label | `cherry-pick/*` | `type/dependencies` |
| DCO filtering | Filters out DCO checks | **Respects DCO** (must pass) |
| PR tracking | Per-branch status | Single PR status |
| PR discovery | Merged PRs with labels | **Open PRs** with label |
| AI assistant | Required for conflicts | Not needed |

### Commands (cmd/dep-merger/)

All commands are in the `main` package within `cmd/dep-merger/`:

- **config** (`cmd_config.go`): Initialize/update configuration (auto-detects org/repo from git)
- **fetch** (`cmd_fetch.go`): Fetch open PRs with `type/dependencies` label
  - Discovers new dependency PRs and adds them to tracking
  - Updates CI status for existing tracked PRs
  - Marks PRs as merged if no longer open
- **status** (`cmd_status.go`): Display tracked PRs with CI status and suggested commands
  - Shows passing/failing/pending CI status
  - Suggests `retry` for failing PRs, `merge` for passing PRs
  - `--fetch` flag to refresh data before displaying
  - `--show-merged` flag to include merged PRs
- **retry** (`cmd_retry.go`): Retry failed CI workflows via GitHub Actions API
  - `dep-merger retry` - Retry all PRs with failing CI
  - `dep-merger retry 123` - Retry specific PR
- **merge** (`cmd_merge.go`): Squash merge PRs with passing CI
  - `dep-merger merge` - Merge all PRs with passing CI
  - `dep-merger merge 123` - Merge specific PR

### Dep-Merger Configuration File

`dep-merger.yaml` schema:
```yaml
org: string
repo: string
last_fetch_date: time.Time
tracked_prs:
  - number: int
    title: string
    ci_status: passing|failing|pending|unknown
    run_attempt: int  # Number of CI retry attempts
    merged: bool
```

### Shared Infrastructure

Dep-merger reuses the following from cherry-picker:

- `internal/github/client.go`: GitHub API client
- `internal/github/workflows.go`: Retry and merge operations
- `internal/github/pr.go`: PR fetching (uses `GetOpenPRsWithLabel`, `GetPRWithDetailsNoDCOFilter`)
- `internal/github/ci_status.go`: CI status checking (with `filterDCO: false`)

---

## Dependencies

- `github.com/spf13/cobra`: CLI framework
- `github.com/google/go-github/v57`: GitHub API client
- `golang.org/x/oauth2`: GitHub authentication
- `gopkg.in/yaml.v3`: YAML parsing
- Go 1.24.6

## Testing

Tests use standard Go testing with dependency injection:
- LoadConfig/SaveConfig functions passed as parameters for mocking
- Pick command has separate `runPickForTest()` method that skips git operations
- Test files: `*_test.go` alongside implementation files

## Environment Variables

- `GITHUB_TOKEN`: Required for GitHub API operations (fine-grained PAT with Contents, PRs, Actions, Issues, Metadata permissions)

## AI Assistant Requirements

- AI assistant CLI tool must be installed and available in PATH
- Must be configured via `--ai-assistant` flag during `config` command
- Tool should support interactive stdin/stdout for conflict resolution
- Recommended options:
  - `cursor-agent`: Install via `npm install -g @anthropic-ai/cursor-agent`
  - `claude`: Install via `npm install -g @anthropic-ai/claude-cli`

## Code Conventions

- Use `exec.Command` for git operations with stdout/stderr piped for visibility
- Commands follow factory pattern: `New<Command>Cmd(dependencies) *cobra.Command`
- Validation uses shared utilities in `internal/commands/common.go`
- DCO check filtering is configurable via `newCIStatusCheckerWithOptions(filterDCO bool)`:
  - Cherry-picker: `filterDCO: true` (ignores DCO failures)
  - Dep-merger: `filterDCO: false` (respects DCO failures)
- The tools expect squash merges for PRs
- Use testify/assert and testify/require when writing or refactoring tests
- Always use the cobracmd Context() or t.Context(), never create one