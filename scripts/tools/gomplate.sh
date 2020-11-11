# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(gomplate)

GOMPLATE_VERSION="3.7.0"

GOMPLATE_SHA256_LINUX="27b0792309b78cd872ffe72a040475b2704f72e055a774079c9fcc5ac23543f6"
GOMPLATE_SHA256_DARWIN="00d0ac833d04a007a510c3fc21e363d873bd06b55c4693e610cd3601abb133d0"
GOMPLATE_SHA256_WINDOWS="83330086ad2f96690cf8c860405973c504b6f00fb7a56f58b6eb003dbd2d931d"

GOMPLATE_URL_LINUX="https://github.com/hairyhenderson/gomplate/releases/download/v{version}/gomplate_linux-amd64"
GOMPLATE_URL_DARWIN="https://github.com/hairyhenderson/gomplate/releases/download/v{version}/gomplate_darwin-amd64"
GOMPLATE_URL_WINDOWS="https://github.com/hairyhenderson/gomplate/releases/download/v{version}/gomplate_windows-amd64.exe"

function gomplate_version { gomplate -v; }
