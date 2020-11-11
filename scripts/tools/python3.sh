# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(python3)

PYTHON3_VERSION=3.3

function python3_version { python3 --version; }

VENV_ACTIVATE="${VENV_DIR}/bin/activate"
if [ ! -f "${VENV_ACTIVATE}" ]; then
    python3 -m venv "${VENV_DIR}"
fi

# Only source the file venv activation once
if [ "${VIRTUAL_ENV:-}" != "${VENV_DIR}" ]; then
    # shellcheck disable=SC1090
    source "${VENV_ACTIVATE}"
fi
