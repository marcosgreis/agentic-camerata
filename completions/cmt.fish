# Fish completion for cmt (Agentic Camerata)

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
complete -c cmt -n "__fish_use_subcommand" -l model -d "Override default model"
complete -c cmt -n "__fish_use_subcommand" -l agent -d "Agent backend (claude, codex, amp)" -r -f -a "claude codex amp"

# Commands
complete -c cmt -n __fish_use_subcommand -a new -d 'Start a new Claude session'
complete -c cmt -n __fish_use_subcommand -a research -d 'Start a research-focused session'
complete -c cmt -n __fish_use_subcommand -a plan -d 'Start a planning session'
complete -c cmt -n __fish_use_subcommand -a implement -d 'Start an implementation session'
complete -c cmt -n __fish_use_subcommand -a review -d 'Review changes in the working directory'
complete -c cmt -n __fish_use_subcommand -a fix-test -d 'Fix a failing test'
complete -c cmt -n __fish_use_subcommand -a fix-local-comments -d 'Look at an issue and fix it'
complete -c cmt -n __fish_use_subcommand -a fix-pr-build -d 'Fix a PR\'s CI build'
complete -c cmt -n __fish_use_subcommand -a fix-pr-comments -d 'Address unresolved PR comments'
complete -c cmt -n __fish_use_subcommand -a quick -d 'Quick single-response query (uses Sonnet)'
complete -c cmt -n __fish_use_subcommand -a play -d 'Run a multi-phase playbook workflow'
complete -c cmt -n __fish_use_subcommand -a sessions -d 'List all sessions'
complete -c cmt -n __fish_use_subcommand -a jump -d 'Jump to a session\'s tmux location'
complete -c cmt -n __fish_use_subcommand -a dashboard -d 'Open the TUI dashboard'
complete -c cmt -n __fish_use_subcommand -a todo -d 'Manage todos'

# File flags for commands that support them
complete -c cmt -n '__fish_seen_subcommand_from new research plan implement review fix-test fix-local-comments fix-pr-build fix-pr-comments' -s f -d 'File path to prepend to prompt (repeatable)' -r -F
complete -c cmt -n '__fish_seen_subcommand_from new research plan implement review fix-test fix-local-comments fix-pr-build fix-pr-comments' -s d -d 'Directory to open fzf file selector on (repeatable)' -r -a '(__fish_complete_directories)'
complete -c cmt -n '__fish_seen_subcommand_from new research plan implement review fix-test fix-local-comments fix-pr-build fix-pr-comments' -s t -d 'Open fzf on thoughts/shared/ directory (repeatable)'

# Loop flags for commands that support them
complete -c cmt -n '__fish_seen_subcommand_from new research plan implement fix-test fix-local-comments fix-pr-build fix-pr-comments' -l loop -d 'Re-run on a recurring interval (e.g. 5m, 1h)' -r -a '1m 5m 10m 30m 1h 2h'
complete -c cmt -n '__fish_seen_subcommand_from new research plan implement fix-test fix-local-comments fix-pr-build fix-pr-comments' -l loop-limit -d 'Maximum number of loop iterations (0 = unlimited)' -r

# new command options
complete -c cmt -n '__fish_seen_subcommand_from new' -s r -l resume -d 'Resume a previous Claude session (interactive picker)'
complete -c cmt -n '__fish_seen_subcommand_from new' -l resume-id -d 'Resume a specific Claude session by ID' -r

# fix-local-comments command options
complete -c cmt -n '__fish_seen_subcommand_from fix-local-comments' -l comment-tag -d 'Comment tag to search for' -r

# play command - complete with any file or directory
complete -c cmt -n '__fish_seen_subcommand_from play' -F -d 'Playbook file'
complete -c cmt -n '__fish_seen_subcommand_from play' -s r -l resume -d 'Resume an abandoned play session' -f -a '(__cmt_sessions)'

# implement command - complete with markdown files for plan argument
complete -c cmt -n '__fish_seen_subcommand_from implement' -a '(__fish_complete_suffix .md)' -d 'Plan file'

# jump command - complete with session IDs
complete -c cmt -n '__fish_seen_subcommand_from jump' -a 'last' -d 'Jump to most recent session'
complete -c cmt -n '__fish_seen_subcommand_from jump' -a '(__cmt_sessions)' -d 'Session ID'

# sessions command options
complete -c cmt -n '__fish_seen_subcommand_from sessions' -s s -d 'Filter by status' -r -a 'waiting working completed abandoned killed deleted restored'
complete -c cmt -n '__fish_seen_subcommand_from sessions' -s n -d 'Limit number of sessions' -r

# dashboard command options
complete -c cmt -n '__fish_seen_subcommand_from dashboard' -l venues -d 'Open directly to venues view'
complete -c cmt -n '__fish_seen_subcommand_from dashboard' -l todos -d 'Open directly to todos view'
complete -c cmt -n '__fish_seen_subcommand_from dashboard' -l debug -d 'Render dashboard to stdout and exit (for debugging)'

# todo subcommands
complete -c cmt -n '__fish_seen_subcommand_from todo; and not __fish_seen_subcommand_from add list search done undone update rm' -a add -d 'Add a new todo'
complete -c cmt -n '__fish_seen_subcommand_from todo; and not __fish_seen_subcommand_from add list search done undone update rm' -a list -d 'List todos'
complete -c cmt -n '__fish_seen_subcommand_from todo; and not __fish_seen_subcommand_from add list search done undone update rm' -a search -d 'Search todos with filters'
complete -c cmt -n '__fish_seen_subcommand_from todo; and not __fish_seen_subcommand_from add list search done undone update rm' -a done -d 'Mark a todo as done'
complete -c cmt -n '__fish_seen_subcommand_from todo; and not __fish_seen_subcommand_from add list search done undone update rm' -a undone -d 'Mark a todo as not done'
complete -c cmt -n '__fish_seen_subcommand_from todo; and not __fish_seen_subcommand_from add list search done undone update rm' -a update -d 'Update a todo'
complete -c cmt -n '__fish_seen_subcommand_from todo; and not __fish_seen_subcommand_from add list search done undone update rm' -a rm -d 'Remove a todo'

# todo add options
complete -c cmt -n '__fish_seen_subcommand_from add' -s s -l source -d 'Source (e.g. slack, github, email)' -r
complete -c cmt -n '__fish_seen_subcommand_from add' -s c -l channel -d 'Channel (e.g. #engineering)' -r
complete -c cmt -n '__fish_seen_subcommand_from add' -s f -l sender -d 'Sender' -r
complete -c cmt -n '__fish_seen_subcommand_from add' -s u -l url -d 'URL' -r
complete -c cmt -n '__fish_seen_subcommand_from add' -s d -l date -d 'Date (YYYY-MM-DD)' -r
complete -c cmt -n '__fish_seen_subcommand_from add' -s k -l key -d 'Idempotency key for deduplication' -r
complete -c cmt -n '__fish_seen_subcommand_from add' -s m -l full-message -d 'Full message text' -r

# todo list options
complete -c cmt -n '__fish_seen_subcommand_from list' -s s -l status -d 'Filter by status' -r -a 'todo done deleted all'

# todo search options
complete -c cmt -n '__fish_seen_subcommand_from search' -l id -d 'Filter by ID' -r
complete -c cmt -n '__fish_seen_subcommand_from search' -s k -l key -d 'Filter by idempotency key' -r
complete -c cmt -n '__fish_seen_subcommand_from search' -s s -l status -d 'Filter by status' -r -a 'todo done'
complete -c cmt -n '__fish_seen_subcommand_from search' -s u -l url -d 'Filter by URL' -r
complete -c cmt -n '__fish_seen_subcommand_from search' -s f -l sender -d 'Filter by sender' -r
complete -c cmt -n '__fish_seen_subcommand_from search' -l source -d 'Filter by source' -r
complete -c cmt -n '__fish_seen_subcommand_from search' -s d -l include-deleted -d 'Include soft-deleted todos'

# todo update options
complete -c cmt -n '__fish_seen_subcommand_from update' -s S -l summary -d 'New summary' -r
complete -c cmt -n '__fish_seen_subcommand_from update' -s s -l source -d 'Source' -r
complete -c cmt -n '__fish_seen_subcommand_from update' -s c -l channel -d 'Channel' -r
complete -c cmt -n '__fish_seen_subcommand_from update' -s f -l sender -d 'Sender' -r
complete -c cmt -n '__fish_seen_subcommand_from update' -s u -l url -d 'URL' -r
complete -c cmt -n '__fish_seen_subcommand_from update' -s d -l date -d 'Date (YYYY-MM-DD)' -r
complete -c cmt -n '__fish_seen_subcommand_from update' -s m -l full-message -d 'Full message text' -r
