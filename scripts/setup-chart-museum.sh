#!/bin/bash

set -e

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "${SCRIPT_DIR}/helpers.sh"

function wait_for_museum_accessible {
	timeout 1m bash -c "until curl http://chartmuseum.${EPINIO_SYSTEM_DOMAIN} > /dev/null 2>&1; do sleep 1; done"
}

prepare_system_domain

echo "Installing chartmuseum"
helm repo add chartmuseum https://chartmuseum.github.io/charts
helm upgrade --install chartmuseum chartmuseum/chartmuseum  \
	--set ingress.enabled=true \
	--set ingress.hosts[0].name="chartmuseum.${EPINIO_SYSTEM_DOMAIN}" \
	--set env.open.DISABLE_API=false \
	--wait

echo "Waiting for chartmuseum to be accessible"
wait_for_museum_accessible

# We need the helm push plugin to automatically package and push chart to our repo
helm plugin install https://github.com/chartmuseum/helm-push.git || true

# Add our new ephemeral repo
helm repo add epinio-chartmuseum "http://chartmuseum.${EPINIO_SYSTEM_DOMAIN}"

pushd ${SCRIPT_DIR}/../helm-charts/
helm cm-push -f --version "0.1.0" chart/container-registry/ epinio-chartmuseum
helm cm-push -f --version "0.1.0" chart/epinio/ epinio-chartmuseum
helm cm-push -f --version "0.1.0" chart/epinio-installer/ epinio-chartmuseum
popd
