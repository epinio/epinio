#!/bin/bash
# syntax: "$@" : additional arguments for the `helm upgrade` command, to customize installation.

set -ex

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "${SCRIPT_DIR}/helpers.sh"

function wait_for_museum_accessible {
  timeout 1m bash -c "until curl ${CHARTMUSEUM_URL} ; do sleep 1; done"
  echo "Waiting ended"
}

# Set chartmuseum URL
if (( PUBLIC_CLOUD == 1)); then
  HOST_IP=$(./scripts/get-runner-ip.sh)
  CHARTMUSEUM_PORT="8080"
  CHARTMUSEUM_URL="http://${HOST_IP}:${CHARTMUSEUM_PORT}"
  INSTALL_OPTS="--set ingress.enabled=false"
else
  prepare_system_domain
  CHARTMUSEUM_URL="http://chartmuseum.${EPINIO_SYSTEM_DOMAIN}"
  INSTALL_OPTS="--set ingress.enabled=true \
                --set ingress.hosts[0].name=\"chartmuseum.${EPINIO_SYSTEM_DOMAIN}\""
fi

echo "Installing chartmuseum"
helm repo add chartmuseum https://chartmuseum.github.io/charts
helm upgrade --install chartmuseum chartmuseum/chartmuseum  \
	${INSTALL_OPTS} \
	--set env.open.DISABLE_API=false \
	"$@" \
	--wait

sleep 5
# look at the service config for debug
kubectl get svc --namespace default chartmuseum -o json

# Configured the port forwarding if needed
if (( PUBLIC_CLOUD == 1)); then
  POD_NAME=$(kubectl get pods --namespace default -l "app.kubernetes.io/name=chartmuseum" -o jsonpath="{.items[0].metadata.name}")
  setsid -f kubectl port-forward --namespace default --address ${HOST_IP} ${POD_NAME} ${CHARTMUSEUM_PORT}:${CHARTMUSEUM_PORT} >/dev/null 2>&1
fi

echo "Waiting for chartmuseum to be accessible"
wait_for_museum_accessible

# We need the helm push plugin to automatically package and push chart to our repo
helm plugin install https://github.com/chartmuseum/helm-push.git || true

# Add our new ephemeral repo
helm repo add --force-update epinio-chartmuseum ${CHARTMUSEUM_URL}

pushd ${SCRIPT_DIR}/../helm-charts/
helm cm-push -f --version "0.1.0" chart/container-registry/ epinio-chartmuseum
helm cm-push -f --version "0.1.0" chart/epinio/ epinio-chartmuseum
helm cm-push -f --version "0.1.0" chart/epinio-installer/ epinio-chartmuseum
popd
