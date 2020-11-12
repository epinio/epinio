# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(stern)

STERN_VERSION="1.11.0"

STERN_URL_DARWIN="https://github.com/wercker/stern/releases/download/{version}/stern_darwin_amd64"
STERN_URL_LINUX="https://github.com/wercker/stern/releases/download/{version}/stern_linux_amd64"
STERN_URL_WINDOWS="https://github.com/wercker/stern/releases/download/{version}/stern_windows_amd64.exe"

STERN_SHA256_DARWIN="7aea3b6691d47b3fb844dfc402905790665747c1e6c02c5cabdd41994533d7e9"
STERN_SHA256_LINUX="e0b39dc26f3a0c7596b2408e4fb8da533352b76aaffdc18c7ad28c833c9eb7db"
STERN_SHA256_WINDOWS="75708b9acf6ef0eeffbe1f189402adc0405f1402e6b764f1f5152ca288e3109e"

function stern_version { stern --version; }
