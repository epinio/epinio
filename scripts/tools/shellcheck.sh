# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(shellcheck)

SHELLCHECK_VERSION="0.7.0"

SHELLCHECK_URL_DARWIN="https://github.com/koalaman/shellcheck/releases/download/v{version}/shellcheck-v{version}.darwin.x86_64.tar.xz"
SHELLCHECK_URL_LINUX="https://github.com/koalaman/shellcheck/releases/download/v{version}/shellcheck-v{version}.linux.x86_64.tar.xz"
SHELLCHECK_URL_WINDOWS="https://github.com/koalaman/shellcheck/releases/download/v{version}/shellcheck-v{version}.zip"

SHELLCHECK_SHA256_DARWIN="c4edf1f04e53a35c39a7ef83598f2c50d36772e4cc942fb08a1114f9d48e5380"
SHELLCHECK_SHA256_LINUX="39c501aaca6aae3f3c7fc125b3c3af779ddbe4e67e4ebdc44c2ae5cba76c847f"
SHELLCHECK_SHA256_WINDOWS="02cfa14220c8154bb7c97909e80e74d3a7fe2cbb7d80ac32adcac7988a95e387"

function shellcheck_version { shellcheck --version; }
