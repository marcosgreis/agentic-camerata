#compdef cmt
# Zsh completion for cmt (Claude Management Tool)

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
        'fix-test:Fix a failing test'
        'look-and-fix:Look at an issue and fix it'
        'quick:Quick single-response query (uses Haiku)'
        'sessions:List all sessions'
        'jump:Jump to a session'\''s tmux location'
        'dashboard:Open the TUI dashboard'
    )

    local -a global_opts
    global_opts=(
        '(-d --db)'{-d,--db}'[Database path]:path:_files'
        '(-v --verbose)'{-v,--verbose}'[Enable verbose output]'
        '(-a --autonomous)'{-a,--autonomous}'[Enable autonomous mode (skip permission prompts)]'
        '(-h --help)'{-h,--help}'[Show help]'
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
                fix-test)
                    _arguments \
                        $file_opts \
                        '1:test:'
                    ;;
                look-and-fix)
                    _arguments \
                        $file_opts \
                        '1:issue:'
                    ;;
                quick)
                    _arguments '1:prompt:'
                    ;;
                sessions)
                    _arguments \
                        '-s[Filter by status]:status:(waiting working completed abandoned)' \
                        '-n[Limit number of sessions]:limit:'
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
                    ;;
            esac
            ;;
    esac
}

_cmt "$@"
