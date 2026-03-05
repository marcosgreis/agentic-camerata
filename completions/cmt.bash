#!/bin/bash
# Bash completion for cmt (Agentic Camerata)

_cmt_completions() {
    local cur prev
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    local commands="new research plan implement fix-test look-and-fix quick play sessions jump dashboard todo"
    local global_opts="-d --db -v --verbose -a --autonomous -h --help"
    local file_opts="-f --files -d --dirs -t --thoughts"

    # Handle command-specific completions
    case "${COMP_WORDS[1]}" in
        new)
            case "$prev" in
                -f|--files)
                    COMPREPLY=($(compgen -f -- "$cur"))
                    ;;
                -d|--dirs)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    ;;
                --resume-id)
                    COMPREPLY=()
                    ;;
                *)
                    if [[ "$cur" == -* ]]; then
                        COMPREPLY=($(compgen -W "$file_opts -r --resume --resume-id" -- "$cur"))
                    fi
                    ;;
            esac
            ;;
        research)
            # research <topic> [-f file] [-d dir] [-t]
            case "$prev" in
                -f|--files)
                    COMPREPLY=($(compgen -f -- "$cur"))
                    ;;
                -d|--dirs)
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
                -f|--files)
                    COMPREPLY=($(compgen -f -- "$cur"))
                    ;;
                -d|--dirs)
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
                -f|--files)
                    COMPREPLY=($(compgen -f -- "$cur"))
                    ;;
                -d|--dirs)
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
                -f|--files)
                    COMPREPLY=($(compgen -f -- "$cur"))
                    ;;
                -d|--dirs)
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
            case "$prev" in
                -f|--files)
                    COMPREPLY=($(compgen -f -- "$cur"))
                    ;;
                -d|--dirs)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    ;;
                --comment-tag)
                    COMPREPLY=()
                    ;;
                *)
                    if [[ "$cur" == -* ]]; then
                        COMPREPLY=($(compgen -W "$file_opts --comment-tag" -- "$cur"))
                    fi
                    ;;
            esac
            ;;
        quick)
            # quick <prompt> - no specific completions
            COMPREPLY=()
            ;;
        play)
            # play <playbook> - complete with markdown files
            if [[ $COMP_CWORD -eq 2 ]]; then
                COMPREPLY=($(compgen -f -X '!*.md' -- "$cur"))
            fi
            ;;
        sessions)
            case "$prev" in
                -s|--status)
                    COMPREPLY=($(compgen -W "waiting working completed abandoned" -- "$cur"))
                    ;;
                -n|--limit)
                    COMPREPLY=()
                    ;;
                *)
                    if [[ "$cur" == -* ]]; then
                        COMPREPLY=($(compgen -W "-s --status -n --limit" -- "$cur"))
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
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "--venues --todos --debug" -- "$cur"))
            fi
            ;;
        todo)
            local todo_commands="add list done undone update rm"
            case "${COMP_WORDS[2]}" in
                add)
                    case "$prev" in
                        -s|--source) COMPREPLY=() ;;
                        -c|--channel) COMPREPLY=() ;;
                        -f|--sender) COMPREPLY=() ;;
                        -u|--url) COMPREPLY=() ;;
                        -d|--date) COMPREPLY=() ;;
                        -k|--key) COMPREPLY=() ;;
                        -m|--full-message) COMPREPLY=() ;;
                        *)
                            if [[ "$cur" == -* ]]; then
                                COMPREPLY=($(compgen -W "-s --source -c --channel -f --sender -u --url -d --date -k --key -m --full-message" -- "$cur"))
                            fi
                            ;;
                    esac
                    ;;
                list)
                    case "$prev" in
                        -s|--status)
                            COMPREPLY=($(compgen -W "todo done all" -- "$cur"))
                            ;;
                        *)
                            if [[ "$cur" == -* ]]; then
                                COMPREPLY=($(compgen -W "-s --status" -- "$cur"))
                            fi
                            ;;
                    esac
                    ;;
                update)
                    case "$prev" in
                        -S|--summary) COMPREPLY=() ;;
                        -s|--source) COMPREPLY=() ;;
                        -c|--channel) COMPREPLY=() ;;
                        -f|--sender) COMPREPLY=() ;;
                        -u|--url) COMPREPLY=() ;;
                        -d|--date) COMPREPLY=() ;;
                        -m|--full-message) COMPREPLY=() ;;
                        *)
                            if [[ "$cur" == -* ]]; then
                                COMPREPLY=($(compgen -W "-S --summary -s --source -c --channel -f --sender -u --url -d --date -m --full-message" -- "$cur"))
                            fi
                            ;;
                    esac
                    ;;
                done|undone|rm)
                    # These take a todo ID - no dynamic completion
                    COMPREPLY=()
                    ;;
                *)
                    if [[ $COMP_CWORD -eq 2 ]]; then
                        COMPREPLY=($(compgen -W "$todo_commands" -- "$cur"))
                    fi
                    ;;
            esac
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
