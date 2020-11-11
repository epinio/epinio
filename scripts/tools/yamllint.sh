# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(yamllint)

YAMLLINT_VERSION=1.23

function yamllint_version { yamllint --version; }
function yamllint_install { python3 -m pip install yamllint; }

YAMLLINT_REQUIRES="python3"
