#!/bin/bash

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
NETWORK_NAME=epinio-acceptance
MIRROR_NAME=epinio-acceptance-registry-mirror
CLUSTER_NAME=epinio-acceptance
export KUBECONFIG=$SCRIPT_DIR/../tmp/acceptance-kubeconfig

check_deps() {
  if ! command -v k3d &> /dev/null
  then
      echo "k3d could not be found"
      exit
  fi
}

existingCluster() {
  k3d cluster list | grep ${CLUSTER_NAME}
}

if [[ "$(existingCluster)" != "" ]]; then
  echo "Cluster already exists, skipping creation."
  exit 0
fi

echo "Ensuring a network"
docker network create $NETWORK_NAME || echo "Network already exists"

echo "Ensuring registry mirror (even if it's stopped)"
existingMirror=$(docker ps -a --filter name=$MIRROR_NAME -q)
if [[ $existingMirror  == "" ]]; then
  echo "No mirror found, creating one"
  docker run -d --network $NETWORK_NAME --name $MIRROR_NAME \
    -e REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io \
    -e REGISTRY_PROXY_USERNAME="${REGISTRY_USERNAME}" \
    -e REGISTRY_PROXY_PASSWORD="${REGISTRY_PASSWORD}" \
    registry:2
else
  docker start $MIRROR_NAME # In case it was stopped (we used "-a" when listing)
fi

echo "Writing epinio config yaml"
TMP_CONFIG="$(mktemp)"
trap "rm -f $TMP_CONFIG" EXIT

cat << EOF > $TMP_CONFIG
mirrors:
  "docker.io":
    endpoint:
      - http://$MIRROR_NAME:5000
EOF

echo "Creating a new one named $CLUSTER_NAME"
if [ -z ${EXPOSE_ACCEPTANCE_CLUSTER_PORTS+x} ]; then
  # Without exposing ports on the host:
  k3d cluster create $CLUSTER_NAME --network $NETWORK_NAME --registry-config $TMP_CONFIG --k3s-server-arg --disable --k3s-server-arg traefik
else
  # Exposing ports on the host:
  k3d cluster create $CLUSTER_NAME --network $NETWORK_NAME --registry-config $TMP_CONFIG -p 80:80@server[0] -p 443:443@server[0] --k3s-server-arg --disable --k3s-server-arg traefik
fi
k3d kubeconfig get $CLUSTER_NAME > $KUBECONFIG

echo "Waiting for node to be ready"
nodeName=$(kubectl get nodes -o name)
kubectl wait --for=condition=Ready ${nodeName}

echo "Done! The cluster is ready."
