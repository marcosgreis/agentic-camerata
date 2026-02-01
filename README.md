# cmt

**Agentic Camerata** — Orchestrate Claude AI coding sessions from your terminal.

```
cmt research "how does auth work in this codebase"
cmt plan "add user settings page"
cmt implement
```

## Features

- **Workflow modes** — Research, plan, implement, and fix with specialized system prompts
- **Session tracking** — SQLite database tracks all your Claude sessions
- **Tmux integration** — Jump back to where you started any session
- **TUI dashboard** — Real-time view of sessions and output
- **Plan file selection** — fzf-based interface for selecting implementation plans

## Requirements

- Go 1.22+
- tmux (required)
- [Claude CLI](https://github.com/anthropics/claude-code) installed and configured
- fzf (required for `implement` command)

## Install

```bash
# Clone and build
git clone https://github.com/marcosgreis/agentic-camerata.git
cd cmt
make build      # Output: ./build/cmt

# Or install directly to GOPATH
make install
```

## Usage

Start tmux first:
```bash
tmux new-session -s dev
```

### Workflow Commands

```bash
# General session
cmt new "help me debug this issue"

# Research mode — focus on understanding, no changes
cmt research "explain the payment flow"

# Plan mode — design approach, get approval before coding
cmt plan "refactor the API layer"

# Implement mode — select a plan file via fzf, then implement
cmt implement
cmt implement path/to/plan.md    # Or specify plan directly

# Fix a failing test
cmt fix-test "TestUserAuth"

# Investigate and fix issues tagged with CMT comments
cmt look-and-fix "authentication bug"

# Quick single-response query (uses Haiku model)
cmt quick "what does this function do?"
```

### Session Management

```bash
# List all sessions
cmt sessions

# List only active sessions
cmt sessions -s active

# Limit number of sessions shown
cmt sessions -n 10

# Jump to where a session was started
cmt jump abc123
cmt jump last
```

### Dashboard

```bash
cmt dashboard
```

| Key | Action |
|-----|--------|
| `j/k` | Navigate sessions |
| `Enter` | Jump to session's tmux pane |
| `i` | Toggle info panel |
| `Tab` | Switch panels |
| `r` | Refresh |
| `q` | Quit |

## Configuration

| Option | Flag | Env | Default |
|--------|------|-----|---------|
| Database | `-d`, `--db` | `CMT_DB` | `~/.config/cmt/sessions.db` |
| Verbose | `-v` | — | `false` |

### Directories

- **Database:** `~/.config/cmt/sessions.db`
- **Session logs:** `~/.config/cmt/output/{session_id}.log`
- **Plan files:** `thoughts/shared/plans/*.md` (for `implement` command)

## Workflow Modes

Each workflow mode injects a system prompt to guide Claude:

| Mode | Command | Focus |
|------|---------|-------|
| **general** | `new` | Open-ended assistance for any task |
| **research** | `research` | Read, explore, understand. No file changes |
| **plan** | `plan` | Design approach, identify trade-offs. Get approval first |
| **implement** | `implement` | Execute a plan file, implement all phases |
| **fix** | `fix-test`, `look-and-fix` | Analyze and fix failing tests or issues |

Session statuses: `active`, `completed`, `abandoned`

## Shell Completions

### Bash
```bash
# Add to ~/.bashrc
source /path/to/cmt/completions/cmt.bash
```

### Zsh
```bash
# Add to ~/.zshrc
fpath=(/path/to/cmt/completions $fpath)
autoload -Uz compinit && compinit
```

### Fish
```bash
# Copy to fish completions directory
cp /path/to/cmt/completions/cmt.fish ~/.config/fish/completions/
```

## Project Structure

```
cmd/cmt/main.go              # Entry point (Kong CLI init, DB open, tmux validation)
completions/                 # Shell completions (bash, zsh, fish)
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
    claude.go                # Session runner, PTY management
    prompts.go               # Workflow prompt prefixes
    terminal.go              # Terminal state management
  db/
    db.go                    # SQLite connection, initialization
    sessions.go              # Session CRUD operations
    schema.sql               # Database schema (embedded)
  plans/
    plans.go                 # Plan file selection via fzf
  tmux/
    tmux.go                  # Tmux detection and navigation
  tui/
    dashboard.go             # Bubble Tea dashboard
    styles.go                # Lipgloss styling
```

## Build & Test

```bash
make build               # Output: ./build/cmt
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

- **Kong** — CLI parsing with struct tags
- **Bubble Tea/Lipgloss** — TUI framework
- **creack/pty** — PTY for interactive sessions
- **modernc.org/sqlite** — Pure Go SQLite (no CGO)
- **google/uuid** — Session ID generation

## License

Apache 2.0
