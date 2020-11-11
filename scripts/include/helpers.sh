# shellcheck shell=bash

function colorize {
    local color=$1
    local text=$2
    # XXX Does not really work correctly for `COLOR= make ...`
    if [ -z "${COLOR:-}" ]; then
        printf "%b\n" "${text}"
    else
        printf "\033[${color}m%b\033[0m\n" "${text}"
    fi
}

function green() {
    colorize 32 "$1"
}

function red() {
    colorize 31 "$1"
}

function blue() {
    colorize 34 "$1"
}

################################################################################

function die {
    # XXX sometimes the error is not displayed; why?
    red "$1"
    exit 1
}

################################################################################

function var_lookup {
    local name
    local default

    name="$(echo "$1" | tr "[:lower:]" "[:upper:]")"
    default="${2:-}"

    eval echo "\${$name:-$default}"
}
