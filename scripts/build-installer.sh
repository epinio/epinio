#!/bin/bash

set -e

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

pushd ${SCRIPT_DIR}/../installer/cmd/epinio-installer > /dev/null
GOARCH="amd64" GOOS="linux" go build -o "${SCRIPT_DIR}/../output/bin/epinio_installer"
popd > /dev/null
