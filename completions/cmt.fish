# Fish completion for cmt (Claude Management Tool)

# Disable file completions by default
complete -c cmt -f

# Helper function to get session IDs
function __cmt_sessions
    cmt sessions 2>/dev/null | tail -n +2 | awk '{print $1}'
end

# Global options
complete -c cmt -s d -l db -d 'Database path' -r
complete -c cmt -s v -l verbose -d 'Enable verbose output'
complete -c cmt -s a -l autonomous -d 'Enable autonomous mode (skip permission prompts)'
complete -c cmt -s h -l help -d 'Show help'

# Commands
complete -c cmt -n __fish_use_subcommand -a new -d 'Start a new Claude session'
complete -c cmt -n __fish_use_subcommand -a research -d 'Start a research-focused session'
complete -c cmt -n __fish_use_subcommand -a plan -d 'Start a planning session'
complete -c cmt -n __fish_use_subcommand -a implement -d 'Start an implementation session'
complete -c cmt -n __fish_use_subcommand -a fix-test -d 'Fix a failing test'
complete -c cmt -n __fish_use_subcommand -a look-and-fix -d 'Look at an issue and fix it'
complete -c cmt -n __fish_use_subcommand -a quick -d 'Quick single-response query (uses Haiku)'
complete -c cmt -n __fish_use_subcommand -a sessions -d 'List all sessions'
complete -c cmt -n __fish_use_subcommand -a jump -d 'Jump to a session\'s tmux location'
complete -c cmt -n __fish_use_subcommand -a dashboard -d 'Open the TUI dashboard'

# File flags for commands that support them (new, research, plan, implement, fix-test, look-and-fix)
complete -c cmt -n '__fish_seen_subcommand_from new research plan implement fix-test look-and-fix' -s f -d 'File path to prepend to prompt (repeatable)' -r -F
complete -c cmt -n '__fish_seen_subcommand_from new research plan implement fix-test look-and-fix' -s d -d 'Directory to open fzf file selector on (repeatable)' -r -a '(__fish_complete_directories)'
complete -c cmt -n '__fish_seen_subcommand_from new research plan implement fix-test look-and-fix' -s t -d 'Open fzf on thoughts/shared/ directory (repeatable)'

# jump command - complete with session IDs
complete -c cmt -n '__fish_seen_subcommand_from jump' -a 'last' -d 'Jump to most recent session'
complete -c cmt -n '__fish_seen_subcommand_from jump' -a '(__cmt_sessions)' -d 'Session ID'

# sessions command options
complete -c cmt -n '__fish_seen_subcommand_from sessions' -s s -d 'Filter by status' -a 'waiting working completed abandoned'
complete -c cmt -n '__fish_seen_subcommand_from sessions' -s n -d 'Limit number of sessions' -r

# implement command - complete with markdown files for plan argument
complete -c cmt -n '__fish_seen_subcommand_from implement' -a '(__fish_complete_suffix .md)' -d 'Plan file'
