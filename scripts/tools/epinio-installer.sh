# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(epinio_installer)

EPINIO_INSTALLER_VERSION="0.0.1"

EPINIO_INSTALLER__URL_DARWIN="https://github.com/epinio/installer/releases/download/v{version}/epinio-installer_darwin_x86_64"
EPINIO_INSTALLER_URL_LINUX="https://github.com/epinio/installer/releases/download/v{version}/epinio-installer_linux_x86_64"
EPINIO_INSTALLER_URL_WINDOWS="https://github.com/epinio/installer/releases/download/v0.0.1-1/epinio-installer_windows_x86_64.zip"


KIND_SHA256_DARWIN="2f62546a8eadac957f4c9cca77064df0a85bbdbdf6c783c56163e4300c3e83fd"
KIND_SHA256_LINUX="27fc05bd820dd9790d0441a2b78b39c59fa717536eb710e627c55388fe39c4c7"
KIND_SHA256_WINDOWS="a8e54ed782030d59b0d0ba7f0e5cf3da42fc58be10eca734c87fd792e73fa3e7"
