#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
TARGET_DIR="${SCRIPT_DIR}/../assets/embedded-files"
WORKDIR="$(mktemp -d)"
trap 'rm -rf -- "$WORKDIR"' EXIT

pushd $WORKDIR
git clone --depth 1 https://github.com/GoogleCloudPlatform/gcp-service-broker.git
cd gcp-service-broker/deployments/helm/gcp-service-broker
helm dependency update
helm package . -d "$TARGET_DIR"
echo $TARGET_DIR
popd
