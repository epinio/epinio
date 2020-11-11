# shellcheck shell=bash
# shellcheck disable=SC2034

TOOLS+=(jq)

JQ_VERSION="1.6"

function jq_version { jq --version; }

JQ_SHA256_DARWIN="5c0a0a3ea600f302ee458b30317425dd9632d1ad8882259fcaf4e9b868b2b1ef"
JQ_SHA256_LINUX="af986793a515d500ab2d35f8d2aecd656e764504b789b66d7e1a0b727a124c44"
JQ_SHA256_WINDOWS="a51d36968dcbdeabb3142c6f5cf9b401a65dc3a095f3144bd0c118d5bb192753"

JQ_URL_DARWIN="https://github.com/stedolan/jq/releases/download/jq-{version}/jq-osx-amd64"
JQ_URL_LINUX="https://github.com/stedolan/jq/releases/download/jq-{version}/jq-linux64"
JQ_URL_WINDOWS="https://github.com/stedolan/jq/releases/download/jq-{version}/jq-win64.exe"
