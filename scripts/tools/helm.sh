# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(helm)

HELM_VERSION="3.6.3"

HELM_URL_DARWIN="https://get.helm.sh/helm-v{version}-darwin-amd64.tar.gz"
HELM_URL_LINUX="https://get.helm.sh/helm-v{version}-linux-amd64.tar.gz"
HELM_URL_WINDOWS="https://get.helm.sh/helm-v{version}-windows-amd64.zip"

HELM_SHA256_DARWIN="84a1ff17dd03340652d96e8be5172a921c97825fd278a2113c8233a4e8db5236"
HELM_SHA256_LINUX="07c100849925623dc1913209cd1a30f0a9b80a5b4d6ff2153c609d11b043e262"
HELM_SHA256_WINDOWS="797d2abd603a2646f2fb9c3fabba46f2fabae5cbd1eb87c20956ec5b4a2fc634"
