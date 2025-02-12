#!/bin/bash
# Copyright © 2021 - 2023 SUSE LLC
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
NETWORK_NAME=epinio-acceptance
MIRROR_NAME=epinio-acceptance-registry-mirror
CLUSTER_NAME=epinio-acceptance
export KUBECONFIG=$SCRIPT_DIR/../tmp/acceptance-kubeconfig
K3S_IMAGE=${K3S_IMAGE:-rancher/k3s:v1.29.2-k3s1}

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

if [[ "$SHARED_REGISTRY_MIRROR" == "" ]]; then
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
else
  echo "Using local registry mirror"
  MIRROR_NAME="$SHARED_REGISTRY_MIRROR"
fi

echo "Writing epinio settings yaml"
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
  k3d cluster create $CLUSTER_NAME --network $NETWORK_NAME --registry-config $TMP_CONFIG --image "$K3S_IMAGE" $EPINIO_K3D_INSTALL_ARGS
else
  # Exposing ports on the host:
  k3d cluster create $CLUSTER_NAME --network $NETWORK_NAME --registry-config $TMP_CONFIG -p '80:80@server:0' -p '443:443@server:0' --image "$K3S_IMAGE" $EPINIO_K3D_INSTALL_ARGS
fi
k3d kubeconfig get $CLUSTER_NAME > $KUBECONFIG

echo "Waiting for node to be ready"
kubectl wait --for=condition=Ready nodes --all --timeout=600s
nodeName=$(kubectl get nodes -o name)
kubectl wait --for=condition=Ready "$nodeName"

date
echo "Waiting for the deployments of the foundational configurations to be ready"
# 1200s = 20 min, to handle even a horrendously slow setup. Regular is 10 to 30 seconds.
kubectl wait --for=condition=Available --namespace kube-system deployment/metrics-server		--timeout=1200s
kubectl wait --for=condition=Available --namespace kube-system deployment/coredns			--timeout=1200s
kubectl wait --for=condition=Available --namespace kube-system deployment/local-path-provisioner	--timeout=1200s
date

echo "Done! The cluster is ready."
