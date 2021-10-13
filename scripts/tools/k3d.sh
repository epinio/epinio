# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(k3d)

K3D_VERSION="5.0.0"

function k3d_version { k3d version; }

K3D_SHA256_DARWIN="e023e0153e08ad5e63a96783021138affbdfbde42c02eb060aaa846d60a9b76c"
K3D_SHA256_LINUX="6744bfd5cea611dc3f2a24da7b25a28fd0dd4b78c486193c238d55619d7b7c43"
K3D_SHA256_WINDOWS="0aa602e2d544e6a3200d0d7cded4eec9441658eb294526aca9d4262d1fe873de"

K3D_URL_DARWIN="https://github.com/rancher/k3d/releases/download/v{version}/k3d-darwin-amd64"
K3D_URL_LINUX="https://github.com/rancher/k3d/releases/download/v{version}/k3d-linux-amd64"
K3D_URL_WINDOWS="https://github.com/rancher/k3d/releases/download/v{version}/k3d-windows-amd64.exe"
