set -e

VERSION="3.9.0"

URL="https://get.helm.sh/helm-v${VERSION}-linux-amd64.tar.gz"
SHA256="1484ffb0c7a608d8069470f48b88d729e88c41a1b6602f145231e8ea7b43b50a"

pushd "$TMP_DIR" > /dev/null
wget -q "$URL" -O "helm.tar.gz"
echo "${SHA256} helm.tar.gz" | sha256sum -c

mkdir -p helm
tar xvf "helm.tar.gz" -C helm
mv helm/*/helm "${OUTPUT_DIR}/helm"
popd > /dev/null
