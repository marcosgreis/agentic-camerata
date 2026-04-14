# Agentic Camerata (cmt)

Terminal orchestrator for Claude AI coding sessions. Manages workflows (research/plan/implement/fix), tracks sessions in SQLite, integrates with tmux for navigation.

## Quick Reference

```
cmt new "task"              # General session
cmt research "topic"        # Read-only exploration
cmt plan "feature"          # Design before implementing
cmt implement [plan_file]   # Execute plan (fzf selector if no file)
cmt fix-test "test"              # Fix a failing test
cmt fix-local-comments "issue"   # Investigate and fix issues
cmt fix-pr-build <PR_LINK>       # Fix CI build for a PR
cmt fix-pr-comments <PR_LINK>    # Address unresolved PR comments
cmt quick "prompt"          # Single-response Sonnet query
cmt play playbook.md        # Run multi-phase playbook workflow
cmt jump [id]               # Navigate to session's tmux location
cmt sessions [-s status]    # List sessions
cmt dashboard               # Interactive TUI
```

## Project Structure

```
cmd/cmt/main.go              # Entry point (Kong CLI init, DB open, tmux validation)
internal/
  cli/
    cli.go                   # Root CLI structure with all command definitions
    new.go                   # General session command
    research.go              # Research workflow
    plan.go                  # Planning workflow
    implement.go             # Implementation with fzf plan selection
    jump.go                  # Tmux navigation
    sessions.go              # List sessions with filtering
    dashboard.go             # TUI dashboard launcher
    quick.go                 # Single-response Sonnet query
    fixtest.go               # Fix failing test workflow
    fix_local_comments.go    # Investigate and fix issues
    fixprbuild.go            # Fix PR CI build workflow
    fixprcomments.go         # Address unresolved PR comments workflow
    play.go                  # Multi-phase playbook workflow
    fileflags.go             # -f/-d/-t file selection flags shared across commands
  claude/
    claude.go                # Runner - session execution, PTY management, activity monitoring
    prompts.go               # CommandType and prompt prefix mappings
    terminal.go              # Terminal state management (raw mode, resize)
  db/
    db.go                    # SQLite connection, database initialization
    sessions.go              # Session CRUD operations (incl. soft delete/restore/prune)
    schema.sql               # Database schema (embedded)
  playbook/
    playbook.go              # Playbook parser (phases from markdown)
  plans/
    plans.go                 # Plan file selection via fzf
  tmux/
    tmux.go                  # Tmux detection, location tracking, navigation
  tui/
    dashboard.go             # Bubble Tea dashboard model and rendering
    venues.go                # Venue aggregation and grid/expanded view rendering
    styles.go                # Lipgloss styling definitions
completions/                 # Shell completions (bash, zsh, fish)
```

## Build & Test

```bash
make build               # ./build/cmt
make install             # Install to GOPATH/bin
make test                # Run all tests
make test-unit           # Unit tests only (skip tmux-dependent)
make test-integration    # Integration tests (requires tmux)
make test-cover          # Generate coverage.html
make test-race           # Run with race detector
make lint                # Run golangci-lint
make fmt                 # Format code
make release             # Build for multiple platforms
```

## Key Dependencies

- **Kong** - CLI parsing (struct tags)
- **Bubble Tea/Lipgloss** - TUI framework
- **creack/pty** - PTY for interactive sessions
- **modernc.org/sqlite** - Pure Go SQLite (no CGO)
- **google/uuid** - Session ID generation

## Database

Default: `~/.config/cmt/sessions.db` (override: `-d` flag or `CMT_DB` env)

**Schema (sessions table):**
- `id` - 8-char UUID (primary key)
- `created_at`, `updated_at` - timestamps
- `workflow_type` - general, research, plan, implement, fix
- `status` - waiting, working, completed, abandoned, killed, deleted, restored
- `working_directory` - where session was started
- `task_description` - the prompt/task
- `prefix` - CMT_PREFIX environment variable value
- `claude_session_id` - Claude CLI session ID
- `tmux_session`, `tmux_window`, `tmux_pane` - tmux location
- `output_file` - path to session log
- `pid` - process ID
- `deleted_at` - soft-delete timestamp (NULL if not deleted)

**Indices:** status, workflow_type, created_at DESC

**Soft delete:** Sessions are soft-deleted (status=deleted, deleted_at set). Pruned after 7 days. Can be restored to status=restored.

## Directories

- **Database:** `~/.config/cmt/sessions.db`
- **Session logs:** `~/.config/cmt/output/{session_id}.log`
- **Plan files:** `thoughts/shared/plans/*.md`
- **Research files:** `thoughts/shared/research/*.md`

## Conventions

- Errors wrapped with context: `fmt.Errorf("operation: %w", err)`
- Session IDs are 8-char truncated UUIDs
- Requires running inside tmux (exits with message if not)
- Database path supports `~` home directory expansion
- WAL mode enabled for SQLite concurrency
- Activity monitoring: session transitions between `waiting` (idle >1s) and `working` (output detected) states
- File selection flags (`-f file`, `-d dir`, `-t`) available on most session commands via `FileFlags`
- Global flags: `-v` (verbose), `-a` (autonomous mode, skips permission prompts; also `CMT_AUTONOMOUS` env)
- Comment tag for fix-local-comments defaults to `CMT` (override with `CMT_COMMENT_TAG` env)

## Workflow Types

| Type | Commands | Prompt |
|------|----------|--------|
| general | `new` | (none) |
| research | `research` | `/research_codebase` |
| plan | `plan` | `/create_plan` |
| implement | `implement` | `/implement_plan implement all phases ignoring any manual verification steps` |
| fix | `fix-test` | `Analyze and fix the failing test at:` |
| fix | `fix-local-comments` | `Take a look at this repo and search for comments tagged with {tag}...` |
| fix | `fix-pr-build` | `Fix the build of the PR I will share and commit with the message 'Fix' and push` |
| fix | `fix-pr-comments` | `Read the unresolved comments from the PR and propose how to fix them` |
| play | `play` | Orchestrates phases from a playbook markdown file (auto-terminates each phase) |

## Session Statuses

- `waiting` - session started, waiting for Claude input
- `working` - Claude is actively generating output
- `completed` - session finished successfully
- `abandoned` - session interrupted/abandoned
- `killed` - session was killed
- `deleted` - soft-deleted (pruned after 7 days)
- `restored` - restored from deleted state

## Dashboard Views

The TUI dashboard (`cmt dashboard`) supports multiple view modes:
- **Normal** - session list with info pane
- **Trash** - soft-deleted sessions
- **Venues** - grid of working directories with session/plan/research counts
- **Venue Expanded** - sessions + plan/research documents for a single venue
