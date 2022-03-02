#!/bin/bash

set -e

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "$SCRIPT_DIR/helpers.sh"


# Ensure we have a value for domain
prepare_system_domain

echo "Installing Epinio UI"
helm upgrade --install epinio-ui --create-namespace -n epinio \
	--set domain="ui.$EPINIO_SYSTEM_DOMAIN" \
  "$SCRIPT_DIR/../helm-charts/chart/epinio-ui" \
  --wait
