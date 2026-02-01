# Agentic Camerata (cmt)

Terminal orchestrator for Claude AI coding sessions. Manages workflows (research/plan/implement/fix), tracks sessions in SQLite, integrates with tmux for navigation.

## Quick Reference

```
cmt new "task"              # General session
cmt research "topic"        # Read-only exploration
cmt plan "feature"          # Design before implementing
cmt implement [plan_file]   # Execute plan (fzf selector if no file)
cmt fix-test "test"         # Fix a failing test
cmt look-and-fix "issue"    # Investigate and fix issues
cmt quick "prompt"          # Single-response Haiku query
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
    quick.go                 # Single-response Haiku query
    fixtest.go               # Fix failing test workflow
    look_and_fix.go          # Investigate and fix issues
  claude/
    claude.go                # Runner - session execution, PTY management
    prompts.go               # CommandType and prompt prefix mappings
    terminal.go              # Terminal state management (raw mode, resize)
  db/
    db.go                    # SQLite connection, database initialization
    sessions.go              # Session CRUD operations
    schema.sql               # Database schema (embedded)
  plans/
    plans.go                 # Plan file selection via fzf
  tmux/
    tmux.go                  # Tmux detection, location tracking, navigation
  tui/
    dashboard.go             # Bubble Tea dashboard model and rendering
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
- `status` - active, completed, abandoned
- `working_directory` - where session was started
- `task_description` - the prompt/task
- `claude_session_id` - Claude CLI session ID
- `tmux_session`, `tmux_window`, `tmux_pane` - tmux location
- `output_file` - path to session log
- `pid` - process ID

**Indices:** status, workflow_type, created_at DESC

## Directories

- **Database:** `~/.config/cmt/sessions.db`
- **Session logs:** `~/.config/cmt/output/{session_id}.log`
- **Plan files:** `thoughts/shared/plans/*.md`

## Conventions

- Errors wrapped with context: `fmt.Errorf("operation: %w", err)`
- Session IDs are 8-char truncated UUIDs
- Requires running inside tmux (exits with message if not)
- Database path supports `~` home directory expansion
- WAL mode enabled for SQLite concurrency

## Workflow Types

| Type | Commands | Prompt |
|------|----------|--------|
| general | `new` | (none) |
| research | `research` | `/research_codebase` |
| plan | `plan` | `/create_plan` |
| implement | `implement` | `/implement_plan implement all phases...` |
| fix | `fix-test`, `look-and-fix` | Custom fix prompts |

## Session Statuses

- `active` - session in progress
- `completed` - session finished successfully
- `abandoned` - session interrupted/abandoned
