set -e

VERSION="5.0.0"

URL="https://github.com/rancher/k3d/releases/download/v${VERSION}/k3d-linux-amd64"
SHA256="6744bfd5cea611dc3f2a24da7b25a28fd0dd4b78c486193c238d55619d7b7c43"

pushd "$TMP_DIR" > /dev/null
wget -q "$URL" -O k3d
echo "${SHA256} k3d" | sha256sum -c

chmod +x k3d
mv k3d "${OUTPUT_DIR}/k3d"
popd > /dev/null
