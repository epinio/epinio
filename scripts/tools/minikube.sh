# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(minikube)

MINIKUBE_VERSION="1.22.0"

MINIKUBE_URL_DARWIN="https://github.com/kubernetes/minikube/releases/download/v{version}/minikube-darwin-amd64"
MINIKUBE_URL_LINUX="https://github.com/kubernetes/minikube/releases/download/v{version}/minikube-linux-amd64"
MINIKUBE_URL_WINDOWS="https://github.com/kubernetes/minikube/releases/download/v{version}/minikube-windows-amd64.exe"

MINIKUBE_SHA256_DARWIN="932a278393cdcb90bff79c4e49d72c1c34910a71010f1466ce92f51d8332fb58"
MINIKUBE_SHA256_LINUX="7579e5763a4e441500e5709eb058384c9cfe9c9dd888b39905b2cdf3d30fbf36"
MINIKUBE_SHA256_WINDOWS="8764ca0e290b4420c5ec82371bcc1b542990a93bdf771578623554be32319d08"
