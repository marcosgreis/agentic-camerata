#compdef cmt
# Zsh completion for cmt (Agentic Camerata)

_cmt_sessions() {
    local sessions
    sessions=(${(f)"$(cmt sessions 2>/dev/null | tail -n +2 | awk '{print $1}')"})
    _describe 'session' sessions
}

_cmt() {
    local -a commands
    commands=(
        'new:Start a new Claude session'
        'research:Start a research-focused session'
        'plan:Start a planning session'
        'implement:Start an implementation session'
        'review:Review changes in the working directory'
        'fix-test:Fix a failing test'
        'fix-local-comments:Look at an issue and fix it'
        'fix-pr-build:Fix a PR'\''s CI build'
        'fix-pr-comments:Address unresolved PR comments'
        'quick:Quick single-response query (uses Sonnet)'
        'play:Run a multi-phase playbook workflow'
        'sessions:List all sessions'
        'jump:Jump to a session'\''s tmux location'
        'dashboard:Open the TUI dashboard'
        'todo:Manage todos'
    )

    local -a global_opts
    global_opts=(
        '(-d --db)'{-d,--db}'[Database path]:path:_files'
        '(-v --verbose)'{-v,--verbose}'[Enable verbose output]'
        '(-a --autonomous)'{-a,--autonomous}'[Enable autonomous mode (skip permission prompts)]'
        '(-h --help)'{-h,--help}'[Show help]'
        '--model[Override default model]:model:'
        '--agent[Agent backend (claude, codex, amp)]:agent:(claude codex amp)'
    )

    local -a file_opts
    file_opts=(
        '*-f[File path to prepend to prompt (repeatable)]:file:_files'
        '*-d[Directory to open fzf file selector on (repeatable)]:directory:_files -/'
        '-t[Open fzf on thoughts/shared/ directory (repeatable)]'
    )

    _arguments -C \
        $global_opts \
        '1:command:->command' \
        '*::arg:->args'

    case $state in
        command)
            _describe 'command' commands
            ;;
        args)
            case $words[1] in
                new)
                    _arguments \
                        $file_opts \
                        '(-r --resume)'{-r,--resume}'[Resume a previous Claude session (interactive picker)]' \
                        '--resume-id[Resume a specific Claude session by ID]:session_id:' \
                        '1:task:'
                    ;;
                research)
                    _arguments \
                        $file_opts \
                        '1:topic:'
                    ;;
                plan)
                    _arguments \
                        $file_opts \
                        '1:task:'
                    ;;
                implement)
                    _arguments \
                        $file_opts \
                        '1:plan:_files -g "*.md"'
                    ;;
                review)
                    _arguments \
                        $file_opts \
                        '1:focus:'
                    ;;
                fix-test)
                    _arguments \
                        $file_opts \
                        '1:test:'
                    ;;
                fix-local-comments)
                    _arguments \
                        $file_opts \
                        '--comment-tag[Comment tag to search for]:tag:' \
                        '1:issue:'
                    ;;
                fix-pr-build)
                    _arguments \
                        $file_opts \
                        '1:pr_link:'
                    ;;
                fix-pr-comments)
                    _arguments \
                        $file_opts \
                        '1:pr_link:'
                    ;;
                quick)
                    _arguments '1:prompt:'
                    ;;
                play)
                    _arguments \
                        '(-r --resume)'{-r,--resume}'[Resume an abandoned play session]:session:_cmt_sessions' \
                        '1:playbook:_files'
                    ;;
                sessions)
                    _arguments \
                        '(-s --status)'{-s,--status}'[Filter by status]:status:(waiting working completed abandoned killed deleted restored)' \
                        '(-n --limit)'{-n,--limit}'[Limit number of sessions]:limit:'
                    ;;
                jump)
                    _arguments '1:session:->sessions'
                    if [[ $state == sessions ]]; then
                        local -a session_opts
                        session_opts=('last:Jump to most recent session')
                        _describe 'session' session_opts
                        _cmt_sessions
                    fi
                    ;;
                dashboard)
                    _arguments \
                        '--venues[Open directly to venues view]' \
                        '--todos[Open directly to todos view]' \
                        '--debug[Render dashboard to stdout and exit (for debugging)]'
                    ;;
                todo)
                    local -a todo_commands
                    todo_commands=(
                        'add:Add a new todo'
                        'list:List todos'
                        'done:Mark a todo as done'
                        'undone:Mark a todo as not done'
                        'update:Update a todo'
                        'rm:Remove a todo'
                    )
                    _arguments -C \
                        '1:todo command:->todo_cmd' \
                        '*::todo arg:->todo_args'
                    case $state in
                        todo_cmd)
                            _describe 'todo command' todo_commands
                            ;;
                        todo_args)
                            case $words[1] in
                                add)
                                    _arguments \
                                        '(-s --source)'{-s,--source}'[Source (e.g. slack, github, email)]:source:' \
                                        '(-c --channel)'{-c,--channel}'[Channel (e.g. #engineering)]:channel:' \
                                        '(-f --sender)'{-f,--sender}'[Sender]:sender:' \
                                        '(-u --url)'{-u,--url}'[URL]:url:' \
                                        '(-d --date)'{-d,--date}'[Date (YYYY-MM-DD)]:date:' \
                                        '(-k --key)'{-k,--key}'[Idempotency key for deduplication]:key:' \
                                        '(-m --full-message)'{-m,--full-message}'[Full message text]:message:' \
                                        '1:summary:'
                                    ;;
                                list)
                                    _arguments \
                                        '(-s --status)'{-s,--status}'[Filter by status]:status:(todo done deleted all)'
                                    ;;
                                done|undone|rm)
                                    _arguments '1:todo ID:'
                                    ;;
                                update)
                                    _arguments \
                                        '(-S --summary)'{-S,--summary}'[New summary]:summary:' \
                                        '(-s --source)'{-s,--source}'[Source]:source:' \
                                        '(-c --channel)'{-c,--channel}'[Channel]:channel:' \
                                        '(-f --sender)'{-f,--sender}'[Sender]:sender:' \
                                        '(-u --url)'{-u,--url}'[URL]:url:' \
                                        '(-d --date)'{-d,--date}'[Date (YYYY-MM-DD)]:date:' \
                                        '(-m --full-message)'{-m,--full-message}'[Full message text]:message:' \
                                        '1:todo ID:'
                                    ;;
                            esac
                            ;;
                    esac
                    ;;
            esac
            ;;
    esac
}

_cmt "$@"
