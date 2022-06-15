set -e

VERSION="1.6"

URL="https://github.com/stedolan/jq/releases/download/jq-${VERSION}/jq-linux64"
SHA256="af986793a515d500ab2d35f8d2aecd656e764504b789b66d7e1a0b727a124c44"

pushd "$TMP_DIR" > /dev/null
wget -q "$URL" -O jq
echo "${SHA256} jq" | sha256sum -c

chmod +x jq
mv jq "${OUTPUT_DIR}/jq"
popd > /dev/null
