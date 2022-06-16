set -e

VERSION="3.6.3"

URL="https://get.helm.sh/helm-v${VERSION}-linux-amd64.tar.gz"
SHA256="07c100849925623dc1913209cd1a30f0a9b80a5b4d6ff2153c609d11b043e262"

pushd "$TMP_DIR" > /dev/null
wget -q "$URL" -O "helm.tar.gz"
echo "${SHA256} helm.tar.gz" | sha256sum -c

mkdir -p helm
tar xvf "helm.tar.gz" -C helm
mv helm/*/helm "${OUTPUT_DIR}/helm"
popd > /dev/null
