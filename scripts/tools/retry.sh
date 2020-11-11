# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(retry)

function retry {
    # Usage: [RETRIES=3] [DELAY=5] retry [command...]
    local max="${RETRIES:-3}"
    local delay="${DELAY:-5}"
    local i=0

    local cr
    local nl
    local isa_tty="false"
    if [ -t 1 ]; then
        isa_tty="true"
        # Output is to a TTY; don't scroll display while waiting.
        cr="\r"
        # Display a trailing space between the last character and the cursor.
        nl=" "
    else
        # Output is to a file; don't overwrite lines.
        cr=""
        nl="\n"
    fi

    local output
    output="$(mktemp)"
    while test "${i}" -lt "${max}" ; do
        printf "${cr}[%2dm %2ds] %s/%s: %s${nl}" \
               "$(( SECONDS / 60 ))" "$(( SECONDS % 60))"\
               "$(( i + 1 ))" "${max}" \
               "$*"

        # Output is printed if command is successful, or after last failed attempt.
        if "$@" &> "${output}"; then
            if "${isa_tty}"; then
                printf "\n"
            else
                # This is mostly just for CI logs.
                cat "${output}"
            fi
            rm "${output}"
            return
        fi
        sleep "${delay}"
        i="$(( i + 1 ))"
    done

    if "${isa_tty}"; then
        printf "\n"
    fi

    cat "${output}"
    rm "${output}"
    return 1
}
