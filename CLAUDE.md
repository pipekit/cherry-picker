# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Cherry-picker is a CLI tool for managing cherry-picks across GitHub repositories, specifically designed for maintaining release branches with semantic versioning. It tracks PR cherry-pick status in a YAML configuration file and integrates with cursor-agent for AI-assisted conflict resolution.

## Build and Test Commands

```bash
# Build the binary
make cherry-picker

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

## Architecture

### Core Components

**main.go**: Entry point using Cobra for CLI commands. All commands have access to a global `--config` flag (default: `cherry-picks.yaml`).

**cmd/config.go**: Defines core data structures:
- `Config`: Repository configuration with org, repo, source branch, AI assistant command, and tracked PRs
- `TrackedPR`: PR tracking with per-branch status
- `BranchStatus`: Status types (pending, failed, picked, merged) and optional PR details
  - `pending`: Bot hasn't attempted cherry-pick yet
  - `failed`: Bot attempted but failed (usually conflicts) - **pick command only works on this status**
  - `picked`: Bot successfully created cherry-pick PR
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
  - **Only works on PRs with `failed` status** (bot attempted but failed)
  - Uses configured AI assistant for interactive conflict resolution
  - Performs git operations and creates cherry-pick PRs
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

## Configuration File Structure

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
- DCO checks are filtered out when determining CI status
- The tool expects squash merges for PRs
