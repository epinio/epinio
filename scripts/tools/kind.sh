# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(kind)

KIND_VERSION="0.6.0"

KIND_URL_DARWIN="https://github.com/kubernetes-sigs/kind/releases/download/v{version}/kind-darwin-amd64"
KIND_URL_LINUX="https://github.com/kubernetes-sigs/kind/releases/download/v{version}/kind-linux-amd64"
KIND_URL_WINDOWS="https://github.com/kubernetes-sigs/kind/releases/download/v{version}/kind-windows-amd64"

KIND_SHA256_DARWIN="eba1480b335f1fd091bf3635dba3f901f9ebd9dc1fb32199ca8a6aaacf69691e"
KIND_SHA256_LINUX="b68e758f5532db408d139fed6ceae9c1400b5137182587fc8da73a5dcdb950ae"
KIND_SHA256_WINDOWS="f022a4800363bd4a0c17ee84b58d3e5f654a945dcaf5f66e2c1c230e417b05fb"
