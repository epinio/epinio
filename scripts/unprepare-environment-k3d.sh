#!/bin/bash
#!/usr/bin/env bash

set -e

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

pushd "${SCRIPT_DIR}/../installer" > /dev/null
"${SCRIPT_DIR}/../output/bin/epinio_installer" uninstall -m assets/examples/manifest.yaml
