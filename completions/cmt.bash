#!/bin/bash
# Bash completion for cmt (Claude Management Tool)

_cmt_completions() {
    local cur prev
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    local commands="new research plan implement fix-test look-and-fix quick sessions jump dashboard"
    local global_opts="-d --db -v --verbose -a --autonomous -h --help"
    local file_opts="-f -d -t"

    # Handle command-specific completions
    case "${COMP_WORDS[1]}" in
        new)
            # new [<task>] [-f file] [-d dir] [-t]
            case "$prev" in
                -f)
                    COMPREPLY=($(compgen -f -- "$cur"))
                    ;;
                -d)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    ;;
                *)
                    if [[ "$cur" == -* ]]; then
                        COMPREPLY=($(compgen -W "$file_opts" -- "$cur"))
                    fi
                    ;;
            esac
            ;;
        research)
            # research <topic> [-f file] [-d dir] [-t]
            case "$prev" in
                -f)
                    COMPREPLY=($(compgen -f -- "$cur"))
                    ;;
                -d)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    ;;
                *)
                    if [[ "$cur" == -* ]]; then
                        COMPREPLY=($(compgen -W "$file_opts" -- "$cur"))
                    fi
                    ;;
            esac
            ;;
        plan)
            # plan <task> [-f file] [-d dir] [-t]
            case "$prev" in
                -f)
                    COMPREPLY=($(compgen -f -- "$cur"))
                    ;;
                -d)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    ;;
                *)
                    if [[ "$cur" == -* ]]; then
                        COMPREPLY=($(compgen -W "$file_opts" -- "$cur"))
                    fi
                    ;;
            esac
            ;;
        implement)
            # implement [<plan>] [-f file] [-d dir] [-t]
            case "$prev" in
                -f)
                    COMPREPLY=($(compgen -f -- "$cur"))
                    ;;
                -d)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    ;;
                *)
                    if [[ "$cur" == -* ]]; then
                        COMPREPLY=($(compgen -W "$file_opts" -- "$cur"))
                    else
                        # Complete with plan files
                        COMPREPLY=($(compgen -f -X '!*.md' -- "$cur"))
                    fi
                    ;;
            esac
            ;;
        fix-test)
            # fix-test <test> [-f file] [-d dir] [-t]
            case "$prev" in
                -f)
                    COMPREPLY=($(compgen -f -- "$cur"))
                    ;;
                -d)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    ;;
                *)
                    if [[ "$cur" == -* ]]; then
                        COMPREPLY=($(compgen -W "$file_opts" -- "$cur"))
                    fi
                    ;;
            esac
            ;;
        look-and-fix)
            # look-and-fix <issue> [-f file] [-d dir] [-t]
            case "$prev" in
                -f)
                    COMPREPLY=($(compgen -f -- "$cur"))
                    ;;
                -d)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    ;;
                *)
                    if [[ "$cur" == -* ]]; then
                        COMPREPLY=($(compgen -W "$file_opts" -- "$cur"))
                    fi
                    ;;
            esac
            ;;
        quick)
            # quick <prompt> - no specific completions
            COMPREPLY=()
            ;;
        sessions)
            # sessions [-s status] [-n limit]
            case "$prev" in
                -s|--status)
                    COMPREPLY=($(compgen -W "waiting working completed abandoned" -- "$cur"))
                    ;;
                -n)
                    COMPREPLY=()
                    ;;
                *)
                    if [[ "$cur" == -* ]]; then
                        COMPREPLY=($(compgen -W "-s -n" -- "$cur"))
                    fi
                    ;;
            esac
            ;;
        jump)
            # jump <session> - complete with session IDs
            if [[ $COMP_CWORD -eq 2 ]]; then
                local sessions
                sessions=$(cmt sessions 2>/dev/null | tail -n +2 | awk '{print $1}')
                COMPREPLY=($(compgen -W "last $sessions" -- "$cur"))
            fi
            ;;
        dashboard)
            # dashboard - no arguments
            COMPREPLY=()
            ;;
        *)
            # Complete commands and global options
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "$global_opts" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "$commands" -- "$cur"))
            fi
            ;;
    esac
}

complete -F _cmt_completions cmt
