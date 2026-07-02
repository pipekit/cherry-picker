# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This repository builds a **single** CLI tool, `cherry-picker`, with two subsystems:

1. **Cherry-picks**: Manages cherry-picks across release branches with AI-assisted conflict resolution (per-target-branch tracking).
2. **Dependencies**: Manages dependency PRs (those with the `type/dependencies` label) with retry, merge, and approve operations (flat per-PR tracking, lifted into `internal/depmerger`).

Both subsystems share the `internal/github` API layer and are tracked in **one** unified YAML file (default `cherry-picker.yaml`) with `cherry_picks:` and `dependencies:` sections, owned by `internal/state`.

A **`daemon`** command runs a background poller that re-scrapes both subsystems on an interval and writes the state file atomically, so interactive commands (`status`, `merge`, ...) read fresh data instantly. The unified state file is written atomically (temp + rename) and writers serialize via an advisory flock on a `<file>.lock` sidecar (`internal/lockfile`); readers are lock-free. A monotonic, PR-keyed merge (`internal/state/merge.go`) prevents a daemon tick from reverting a user action that lands mid-tick.

Commands `fetch`, `status`, `merge`, and `retry` are **unified** and act across both subsystems (`merge`/`retry` dispatch by which section tracks the PR number, applying the correct DCO policy). `pick`/`summary` are cherry-pick only; `approve` is dependencies only. Use `cherry-picker migrate` to build the unified file from legacy `cherry-picks.yaml` + `dep-merger.yaml`.

## Build and Test Commands

```bash
# Build the binary
make build

# Run all checks (format, vet, test, lint)
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

**main.go**: Entry point using Cobra for CLI commands. All commands have access to a global `--config` flag (default: `cherry-picker.yaml`). The cherry-pick-only commands (`config`, `pick`, `summary`) are wired to the unified state file via adapter closures in `adapters.go` (`loadCherry`/`saveCherry` project the `cherry_picks:` section to/from `cmd.Config`). The unified `fetch`/`status`/`merge`/`retry`/`approve`/`migrate`/`daemon` commands live in the root `main` package (`cmd_*.go`).

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

**internal/github/client.go**: GitHub API wrapper using `google/go-github/v80`:
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

## Unified Configuration File

`cherry-picker.yaml` schema (owned by `internal/state`; `internal/config` is retained only as a legacy unmarshal shim for `migrate`):
```yaml
org: string
repo: string
last_fetch_date: time.Time
cherry_picks:
  source_branch: string
  ai_assistant_command: string  # Required for the pick command
  last_checked_release: {<branch>: <tag>}
  tracker_issues: {<branch>: <issue-number>}
  tracked_prs:
    - number: int
      title: string
      branches:
        <branch-name>:
          status: pending|failed|picked|merged|released
          pr:  # Only present for picked/merged status
            number: int
            title: string
            ci_status: passing|failing|pending|unknown
dependencies:
  tracked_prs:
    - number: int
      title: string
      ci_status: passing|failing|pending|unknown
      run_attempt: int
      approved: bool
      merged: bool
```

Run `cherry-picker migrate` (idempotent) to build this file from legacy `cherry-picks.yaml` + `dep-merger.yaml`; the legacy files are left in place.

---

## Dependencies Subsystem (`internal/depmerger`)

Formerly the standalone `dep-merger` binary, now the `internal/depmerger` package. Each operation takes an injected `*github.Client` and mutates a `*depmerger.Config` in memory; persistence is owned by the caller (unified CLI commands / daemon) via `internal/state`.

### Key Differences from Cherry-Picks

| Aspect | Cherry-Picks | Dependencies |
|--------|--------------|--------------|
| Label | `cherry-pick/*` | `type/dependencies` |
| DCO filtering | Filters out DCO checks | **Respects DCO** (must pass) |
| PR tracking | Per-branch status | Single PR status |
| PR discovery | Merged PRs with labels | **Open PRs** with label |
| AI assistant | Required for conflicts | Not needed |

### Exported operations (`internal/depmerger`)

- `RefreshDeps(ctx, client, *Config)` — fetch open `type/dependencies` PRs, update CI/approval, mark closed PRs merged (no file I/O).
- `MergePRs` / `RetryPRs` / `ApprovePRs(ctx, client, *Config, prNumber)` — bulk (prNumber 0) or single-PR operations.
- `RenderStatus(w, *Config, execPath, configFlag, showMerged)` — the dependency section of `status`.
- `FindTrackedPR(*Config, number)` — used by the unified `merge`/`retry` dispatch.

These are invoked by the unified commands in the root `main` package: unified `merge`/`retry` dispatch to the cherry (`cmd/merge`.`Execute` / `cmd/retry`.`Execute`) or dependency path by PR number; `approve` is dependency-only; unified `fetch`/`daemon` call `internal/refresh.All`.

### Shared Infrastructure

Both subsystems share:

- `internal/github/client.go`: GitHub API client
- `internal/github/workflows.go`: Retry and merge operations
- `internal/github/pr.go`: PR fetching (deps use `GetOpenPRsWithLabel`, `GetPRWithDetailsNoDCOFilter`)
- `internal/github/ci_status.go`: CI status checking (deps pass `filterDCO: false`)
- `internal/state`: unified config+state (atomic `Save`, lock-guarded `Update`, monotonic merge)
- `internal/lockfile`: advisory flock on the `<file>.lock` sidecar for writers
- `internal/refresh.All`: orchestrates a full scrape of both subsystems (shared by `fetch` and `daemon`)

---

## Dependencies

- `github.com/spf13/cobra`: CLI framework
- `github.com/google/go-github/v80`: GitHub API client
- `golang.org/x/oauth2`: GitHub authentication
- `golang.org/x/sys/unix`: `flock` for the state-file writer lock
- `gopkg.in/yaml.v3`: YAML parsing
- Go 1.26.4

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

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **cherry-picker** (965 symbols, 3014 relationships, 81 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> Index stale? Run `node .gitnexus/run.cjs analyze` from the project root — it auto-selects an available runner. No `.gitnexus/run.cjs` yet? `npx gitnexus analyze` (npm 11 crash → `npm i -g gitnexus`; #1939).

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows. For regression review, compare against the default branch: `detect_changes({scope: "compare", base_ref: "main"})`.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `context({name: "symbolName"})`.

## Never Do

- NEVER edit a function, class, or method without first running `impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `rename` which understands the call graph.
- NEVER commit changes without running `detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/cherry-picker/context` | Codebase overview, check index freshness |
| `gitnexus://repo/cherry-picker/clusters` | All functional areas |
| `gitnexus://repo/cherry-picker/processes` | All execution flows |
| `gitnexus://repo/cherry-picker/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
