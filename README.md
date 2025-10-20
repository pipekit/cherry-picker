# Cherry Picker

A CLI tool for managing cherry-picks across GitHub repositories using a YAML configuration file to track state.

It is currently highly opinionated and only supports GitHub repositories.
* It uses cursor-agent for AI-assisted conflict resolution.
* It uses semantic versioning for target branches.
* It expects squash merges for PRs.

My use case is to cherry-pick PRs from the main branch to release branches for argoproj/argo-workflows.

I'm open to contributions to make it more flexible and support other versioning schemes and repositories.

## Installation

Build the tool from source:

```bash
make cherry-picker
```

Or run all checks and build:

```bash
make all
```

This will format the code, run tests, and build the binary.

## GitHub Token Setup

This tool requires a GitHub Personal Access Token (PAT) with specific permissions to interact with the GitHub API.

### Required Permissions

For **fine-grained personal access tokens**, the following repository permissions are required:

- **Contents**: Read and Write
- **Issues**: Read and Write
- **Pull requests**: Read and Write  
- **Actions**: Read and Write
- **Metadata**: Read

Fewer permissions can be granted but some features will not work. Issues Read/Write is for a future release/issue tracking feature.

### Token Setup

1. Create a fine-grained personal access token at: https://github.com/settings/personal-access-tokens/new
2. Select the repository you want to manage cherry-picks for
3. Grant the permissions listed above
4. Export the token as an environment variable:

```bash
export GITHUB_TOKEN="your_github_token_here"
```

### Permission Usage

- **Contents (Read+Write)**: Used to read repository files and perform merge operations
- **Issues (Read+Write)**: Used to read PR metadata and update status
- **Pull requests (Read+Write)**: Used to fetch PR details, create cherry-pick PRs, and merge PRs
- **Actions (Read+Write)**: Used to retry failed CI workflows and check CI status
- **Metadata (Read)**: Used to access repository metadata required for API operations

### Troubleshooting Merge Issues

If you can merge PRs in the GitHub UI but the `merge` command fails:

1. **Check PAT Permissions**: Ensure your token has **Contents: Read and Write** permission (most common issue)
2. **Verify Branch Protection**: Repository branch protection rules may block API merges while allowing UI merges
3. **Organization Policies**: Some organizations restrict fine-grained PAT merge capabilities
4. **Test with Classic PAT**: Try a classic PAT with `repo` scope to isolate permission issues

**Common Error**: `403 Forbidden` or `merge not allowed` usually indicates missing **Contents** permission.

## Usage

### Initialize Configuration

Create a new `cherry-picks.yaml` configuration file:

```bash
./cherry-picker config --org myorg --repo myrepo
```

This creates a configuration file with the default source branch set to `main`.

### Fetch New PRs

Fetch merged PRs from GitHub that have cherry-pick labels:

```bash
export GITHUB_TOKEN="your_github_token"
./cherry-picker fetch
```

This command will:

- Fetch merged PRs to the source branch since the last fetch date (or 30 days ago for first run)
- Only include PRs with `cherry-pick/*` labels (e.g., `cherry-pick/3.6` for release-3.6)
- Check PR comments for existing cherry-pick PRs created by bots (e.g., argo-cd-cherry-pick-bot)
- Automatically add PRs to tracking:
  - If bot already created cherry-pick PRs: mark as "picked" or "merged" with CI status
  - Otherwise: mark as "pending" for manual cherry-picking
- Update the last fetch date

### Cherry-Pick PRs

Cherry-pick a PR to specific target branches:

```bash
./cherry-picker pick 123 release-1.0
./cherry-picker pick 123  # Pick to all pending branches
```

This command will:

- Create a new branch (`cherry-pick-<prnum>-<target>`)
- Perform `git cherry-pick -x` with AI-assisted conflict resolution via Cursor
- Push the branch and create a cherry-pick PR
- Update the configuration with the new PR details

### Retry Failed CI

Retry failed CI workflows for picked PRs:

```bash
./cherry-picker retry 123 release-1.0  # Retry specific branch
./cherry-picker retry 123              # Retry all branches with failed CI
```

### Merge PRs

Squash and merge picked PRs with passing CI:

```bash
./cherry-picker merge 123 release-1.0  # Merge specific branch
./cherry-picker merge 123              # Merge all eligible branches
```

This command performs a **squash merge** by default, combining all commits in the PR into a single commit. This matches the "Squash and merge" button in the GitHub UI and works with repositories that have merge commits disabled.

### Check Status

View the current status of all tracked PRs:

```bash
./cherry-picker status
```

### Generate Summary

Generate a development progress summary for a target branch:

```bash
./cherry-picker summary release-1.0
```

This command will:

- Query GitHub for commits to the specified target branch since the last release
- Generate markdown output showing cherry-picked PRs and their status
- Include in-progress items (picked but not yet merged)
- Show open PRs against the target branch
- Determine the next patch version based on existing tags

### Check Status

View the current status of all tracked PRs:

```bash
./cherry-picker status
```

This command will:

- Show all tracked PRs and their status across branches
- Display pending (⏳), picked (✅/🔄), merged (✅) states
- **Fetch PR details from GitHub** (when `GITHUB_TOKEN` is set):
  - PR title and GitHub URL
  - Merge status (✅ merged / ❌ not merged)
  - CI status (✅ passing / ❌ failing / 🔄 pending / ❓ unknown)
- **Show contextual commands** directly under each branch status:
  - **Pending branches**: `pick` command
  - **Picked branches with failing CI**: `retry` command
  - **Picked branches with passing CI**: `merge` command
- Provide a summary of total pending and completed picks

#### Example Output

```bash
Cherry-pick status for myorg/myrepo (source: main)

Fix critical bug (https://github.com/myorg/myrepo/pull/123)
  release-1.0    : ⏳ pending
                   💡 ./cherry-picker pick 123 release-1.0
  release-2.0    : 🔄 picked (https://github.com/myorg/myrepo/pull/456)
                   Fix critical bug (cherry-pick release-2.0) [❌ not merged, ❌ CI failing]
                   💡 ./cherry-picker retry 123 release-2.0

Add new feature (https://github.com/myorg/myrepo/pull/125)
  release-1.0    : ✅ picked (https://github.com/myorg/myrepo/pull/457)
                   Add new feature (cherry-pick release-1.0) [✅ merged, ✅ CI passing]
  release-2.0    : 🔄 picked (https://github.com/myorg/myrepo/pull/458)
                   Add new feature (cherry-pick release-2.0) [❌ not merged, ✅ CI passing]
                   💡 ./cherry-picker merge 125 release-2.0

Summary: 2 PR(s), 1 branch pick(s) pending, 3 branch pick(s) completed (2 picked, 1 merged)
```

**Note:** PR details are only fetched when `GITHUB_TOKEN` environment variable is set. Without it, only PR numbers are shown.

## Command Reference

### config

Initialize or update configuration:

- `--org, -o`: GitHub organization or username (auto-detected from git if available)
- `--repo, -r`: GitHub repository name (auto-detected from git if available)  
- `--source-branch, -s`: Source branch name (auto-detected from git if available, defaults to "main")
- `--config, -c`: Configuration file path (default: "cherry-picks.yaml")

Target branches are automatically determined from `cherry-pick/*` labels on PRs.

### fetch

Fetch merged PRs with cherry-pick labels:

- `--config, -c`: Configuration file path (default: "cherry-picks.yaml")
- `--since, -s`: Fetch PRs since this date (YYYY-MM-DD), defaults to last fetch date

PRs are automatically added based on their `cherry-pick/*` labels. For example, a PR with label `cherry-pick/3.6` will be tracked for branch `release-3.6`.

### pick

Cherry-pick a PR to target branches:

- `--config, -c`: Configuration file path (default: "cherry-picks.yaml")

### retry

Retry failed CI workflows:

- `--config, -c`: Configuration file path (default: "cherry-picks.yaml")

### merge

Squash and merge picked PRs:

- `--config, -c`: Configuration file path (default: "cherry-picks.yaml")

### status

View current status of tracked PRs:

- `--config, -c`: Configuration file path (default: "cherry-picks.yaml")

### summary

Generate development progress summary for a target branch:

- `--config, -c`: Configuration file path (default: "cherry-picks.yaml")

#### Examples

Initialize with custom source branch:

```bash
./cherry-picker config --org myorg --repo myrepo --source-branch develop
```

Initialize with custom config file:

```bash
./cherry-picker config --config my-picks.yaml --org myorg --repo myrepo
```

Fetch PRs since a specific date:

```bash
./cherry-picker fetch --since 2024-01-01
```

## AI-Assisted Conflict Resolution

When a cherry-pick encounters merge conflicts, the tool launches an interactive AI session to help you resolve them:

### How It Works

1. **Automatic Detection**: When `git cherry-pick` fails due to conflicts, the tool detects this automatically
2. **Initial Context**: Sends a detailed prompt to the AI about the specific conflicts and cherry-pick context
3. **Interactive Handover**: After providing context, you take control of the conversation
4. **Guided Resolution**: Work with the AI to understand and resolve conflicts step-by-step
5. **User Decision**: After the AI session, you decide whether to proceed with the cherry-pick

### Interactive Benefits

The initial context + interactive approach gives you:
- **Informed AI**: AI starts with full context about the cherry-pick and conflicts
- **Direct Communication**: Ask specific questions and guide the resolution
- **Educational Value**: Learn why conflicts occurred and how to resolve them  
- **Flexible Approach**: Guide the AI toward your preferred resolution strategy
- **Real-time Feedback**: See and approve changes before they're applied

### Requirements

- **Cursor installed**: The `cursor-agent` CLI must be available
- **Authentication**: Run `cursor-agent login` to authenticate if needed
- **Git Context**: The AI can see your repository state and conflicted files

### Example Workflow

```bash
# When conflicts occur during cherry-pick:
./cherry-picker pick 123 release-1.0

# Output:
🎯 Cherry-pick PR #123: Fix critical bug to branch: release-1.0
⚠️  Cherry-pick conflicts detected. Attempting Cursor AI-assisted resolution...
📋 Found 2 conflicted file(s): [src/main.go src/config.go]
🤖 Launching cursor-agent with initial context...
💡 Starting AI session with conflict context, then handing control to you.
   - The AI will receive details about the cherry-pick conflicts
   - You can then guide the resolution process
   - Exit the agent when you're satisfied with the resolution

🎯 Sending initial context to AI...

# AI receives this context automatically:
# "I need help resolving cherry-pick conflicts. Here's the situation:
# 
# **Cherry-pick Context:**
# - Attempting to cherry-pick: 448c492 Fix critical bug
# - Number of conflicted files: 2
# - Conflicted files: [src/main.go src/config.go]
# 
# Please start by examining the conflicted files and let me know what you see."

# Then you take control of the conversation:
# > "What conflicts do you see in main.go?"
# > "Can you resolve the conflicts in config.go first?"
# > "Please make the changes to merge both approaches"

# After you exit the cursor-agent session:
🔍 Cursor-agent session completed.
   - Assuming conflicts have been resolved during the AI session
   - Checking if cherry-pick is complete...
🎯 No conflicts remaining. Completing cherry-pick commit...
✅ Cherry-pick completed with AI-assisted conflict resolution
```

### Post-Session Workflow

After the interactive cursor-agent session, the tool automatically:

1. **Checks for remaining conflicts**: Verifies no conflict markers remain in files
2. **Detects completion status**: Checks if the AI already completed the cherry-pick
3. **Completes the commit**: Runs `git cherry-pick --continue` if needed
4. **Proceeds with PR creation**: Continues with pushing the branch and creating the PR

If conflicts still remain after the AI session:
- The tool will list the problematic files
- You'll need to resolve them manually and run `git cherry-pick --continue`
- Or run `git cherry-pick --abort` to cancel

### Interactive Session Tips

For effective AI conflict resolution:

- **Be Specific**: Ask for help with particular files or conflict types
- **Explain Context**: Tell the AI about the purpose of the changes being cherry-picked  
- **Review Changes**: Check each file as the AI modifies it
- **Ask Questions**: Use the AI to understand why conflicts occurred
- **Iterate**: Work through complex conflicts step-by-step with the AI

### Fallback Behavior

When the interactive AI session encounters issues:

1. **Session Failure**: If `cursor-agent` fails to launch, you'll get clear instructions for manual resolution
2. **User Control**: You can exit the AI session at any time and handle conflicts manually
3. **Git State**: Cherry-pick remains in progress, allowing you to continue with standard Git tools
4. **No Automated Changes**: The interactive approach doesn't make changes without your approval

If `cursor-agent` is not available, the cherry-pick process will abort with a clear error message:

**Example failure output:**
```bash
❌ Failed to launch cursor-agent: exec: "cursor-agent": executable file not found in $PATH
   - You can resolve conflicts manually using standard Git tools
   - Run 'git cherry-pick --abort' to cancel, or resolve and 'git cherry-pick --continue'
```

## Configuration File

The `cherry-picks.yaml` file stores the repository configuration and tracked PRs:

```yaml
org: myorg
repo: myrepo
source_branch: main
last_fetch_date: 2024-01-15T10:30:00Z
tracked_prs:
  - number: 123
    title: "Fix critical bug"
    branches:
      release-1.0:
        status: pending
      release-2.0:
        status: merged
        pr:
          number: 456
          title: "Fix critical bug (cherry-pick release-2.0)"
          ci_status: "passing"
  - number: 124
    title: "Add new feature"
    branches:
      release-1.0:
        status: picked
        pr:
          number: 457
          title: "Add new feature (cherry-pick release-1.0)"
          ci_status: "passing"
```

### PR Status Tracking

Each tracked PR has per-branch status tracking:

- **number**: Original PR number
- **title**: PR title (fetched from GitHub)
- **branches**: Map of target branch names to their status:
  - **status**: `pending`, `picked`, or `merged`
  - **pr**: Details of the cherry-pick PR (when status is `picked` or `merged`):
    - **number**: Cherry-pick PR number
    - **title**: Cherry-pick PR title
    - **ci_status**: CI status (`passing`, `failing`, `pending`, `unknown`)

## Development

Run all checks (format, vet, test):

```bash
make check
```

Build the binary:

```bash
make cherry-picker
```

Clean build artifacts:

```bash
make clean
```
