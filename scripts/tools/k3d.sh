# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(k3d)

K3D_VERSION="4.4.4"

function k3d_version { k3d version; }

K3D_SHA256_DARWIN="5fc9b68c9cd523ea743a9ca31163330db642d50f4db421ee00f7e1f4a29da552"
K3D_SHA256_LINUX="6d4ac3d4c5b084f445980e427c5d3a75eefd2c39a22d028825c234c6c20d1e46"
K3D_SHA256_WINDOWS="1e09e00ee830247ededc3c4718e62f89c1ec7a634fcdf9defb76d124e6a43b83"

K3D_URL_DARWIN="https://github.com/rancher/k3d/releases/download/v{version}/k3d-darwin-amd64"
K3D_URL_LINUX="https://github.com/rancher/k3d/releases/download/v{version}/k3d-linux-amd64"
K3D_URL_WINDOWS="https://github.com/rancher/k3d/releases/download/v{version}/k3d-windows-amd64.exe"
