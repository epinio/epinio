#!/usr/bin/env bash
source scripts/include/setup.sh

# Add additional tool dependencies from the internal tool definitions.
for tool in "${TOOLS[@]}"; do
    # shellcheck disable=SC2207
    TOOLS+=($(var_lookup "${tool}_requires"))
done

# Sort tools alphabetically and remove duplicates.
# shellcheck disable=SC2207
TOOLS=($(printf '%s\n' "${TOOLS[@]}" | sort | uniq))

RC=0
for tool in "${TOOLS[@]}"; do
    if ! STATUS="$(tool_status "${tool}")"; then
        RC=1
    fi

    # Don't show internal tools unless VERBOSE is set.
    if [[ -n "${VERBOSE:-}" || ! "${STATUS}" =~ is[[:space:]]internal ]]; then
        echo "${STATUS}"
    fi
done

if [ "${RC}" -eq 1 ]; then
    echo
    die "Some tools are missing or too old"
fi
