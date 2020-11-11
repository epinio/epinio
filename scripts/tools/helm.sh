# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(helm)

HELM_VERSION="3.3.0"

HELM_URL_DARWIN="https://get.helm.sh/helm-v{version}-darwin-amd64.tar.gz"
HELM_URL_LINUX="https://get.helm.sh/helm-v{version}-linux-amd64.tar.gz"
HELM_URL_WINDOWS="https://get.helm.sh/helm-v{version}-windows-amd64.zip"

HELM_SHA256_DARWIN="3399430b0fdfa8c840e77ddb4410d762ae64f19924663dbdd93bcd0e22704e0b"
HELM_SHA256_LINUX="ff4ac230b73a15d66770a65a037b07e08ccbce6833fbd03a5b84f06464efea45"
HELM_SHA256_WINDOWS="1bac19768e853ada10b9ee7896678a5a7352cc06546c5ea3d47652fcea1033c3"
