# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(k3d)

K3D_VERSION="4.1.1"

function k3d_version { k3d version; }

K3D_SHA256_DARWIN="40fe273f567bad7e031636fe896a05637f173f047ee41f9566a6ad933e7da34f"
K3D_SHA256_LINUX="4148b8035774a2765969898cc73066491ac718d1b2890415a4625931aaacd3ac"
K3D_SHA256_WINDOWS="dc13acba412b473ab5b602420bf61cb2149a8b99cc24a0261574e3f965f03560"

K3D_URL_DARWIN="https://github.com/rancher/k3d/releases/download/v{version}/k3d-darwin-amd64"
K3D_URL_LINUX="https://github.com/rancher/k3d/releases/download/v{version}/k3d-linux-amd64"
K3D_URL_WINDOWS="https://github.com/rancher/k3d/releases/download/v{version}/k3d-windows-amd64.exe"
