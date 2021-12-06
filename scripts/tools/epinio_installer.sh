# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(epinio_installer)

EPINIO_INSTALLER_VERSION="0.0.1-12"

EPINIO_INSTALLER__URL_DARWIN="https://github.com/epinio/installer/releases/download/v{version}/epinio-installer_darwin_x86_64"
EPINIO_INSTALLER_URL_LINUX="https://github.com/epinio/installer/releases/download/v{version}/epinio-installer_linux_x86_64"
EPINIO_INSTALLER_URL_WINDOWS="https://github.com/epinio/installer/releases/download/v{version}/epinio-installer_windows_x86_64.zip"

EPINIO_INSTALLER_SHA256_DARWIN="d0d756c6895663ef6022a83ec393cf4527a007edbbabae027a653ed38b010b96"
EPINIO_INSTALLER_SHA256_LINUX="fab96a6ca39a681373108ce4396d2f8bf79f6ebea1e6e17f6890f7f8f2f90850"
EPINIO_INSTALLER_SHA256_WINDOWS="c607b30f8aec46d4dc885c9e0599c91b5f4492641c60e9308991420110952f18"
