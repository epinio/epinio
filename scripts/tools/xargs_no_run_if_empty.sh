# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(xargs_no_run_if_empty)

function xargs_no_run_if_empty {
    if [ "${UNAME}" = "DARWIN" ]; then
        # macOS xargs doesn't support --no-run-if-empty; it is default behavior.
        xargs "$@"
    else
        xargs --no-run-if-empty "$@"
    fi
}

XARGS_NO_RUN_IF_EMPTY_REQUIRES="xargs"
