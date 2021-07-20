# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(kind)

KIND_VERSION="0.11.1"

KIND_URL_DARWIN="https://github.com/kubernetes-sigs/kind/releases/download/v{version}/kind-darwin-amd64"
KIND_URL_LINUX="https://github.com/kubernetes-sigs/kind/releases/download/v{version}/kind-linux-amd64"
KIND_URL_WINDOWS="https://github.com/kubernetes-sigs/kind/releases/download/v{version}/kind-windows-amd64"

KIND_SHA256_DARWIN="432bef555a70e9360b44661c759658265b9eaaf7f75f1beec4c4d1e6bbf97ce3"
KIND_SHA256_LINUX="949f81b3c30ca03a3d4effdecda04f100fa3edc07a28b19400f72ede7c5f0491"
KIND_SHA256_WINDOWS="d309d8056cec8bcabb24e185200ef8f9702e0c01a9ec8a7f7185fe956783ed97"
