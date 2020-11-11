# shellcheck shell=bash

set -o errexit -o nounset -o pipefail

source scripts/include/defaults.sh
source scripts/include/helpers.sh
source scripts/include/tools.sh
source scripts/include/versions.sh

# COLOR defaults to true if stdout is a tty.
if [[ -z "${COLOR:-}" && -t 1 ]]; then
    COLOR=true
fi

if [ -n "${XTRACE:-}" ]; then
    set -o xtrace
fi

# NO_PINNED_TOOLS exists only for debugging tooling scripts.
if [ -z "${NO_PINNED_TOOLS:-}" ]; then
    export PATH="${TOOLS_DIR}:${PATH}"
fi
