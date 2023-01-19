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

check_deps() {
  if ! command -v kind &> /dev/null
  then
      echo "kind could not be found"
      exit
  fi
}

existingCluster() {
  kind get clusters | grep ${CLUSTER_NAME}
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
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
  - |-
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
      endpoint = ["http://${MIRROR_NAME}:5000"]
EOF

if [ -n ${EXPOSE_ACCEPTANCE_CLUSTER_PORTS+x} ]; then
  cat << EOF >> $TMP_CONFIG
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
  - containerPort: 443
    hostPort: 443
EOF
fi

echo "Creating a new one named $CLUSTER_NAME"
kind create cluster --name $CLUSTER_NAME  --config=$TMP_CONFIG

echo "Connecting the kind cluster to the mirror network"
docker network connect $NETWORK_NAME "${CLUSTER_NAME}-control-plane"

kind get kubeconfig --name $CLUSTER_NAME > $KUBECONFIG

echo "Waiting for node to be ready"
nodeName=$(kubectl get nodes -o name)
kubectl wait --for=condition=Ready "$nodeName"

# Setup LoadBalancer
# https://github.com/epinio/epinio/blob/main/docs/user/howtos/provision_external_ip_for_local_kubernetes.md
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.9.5/manifests/namespace.yaml
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.9.5/manifests/metallb.yaml
kubectl create secret generic -n metallb-system memberlist --from-literal=secretkey="$(openssl rand -base64 128)"

SUBNET_IP=`docker network inspect kind | jq -r '.[0].IPAM.Config[0].Gateway'`
## Use the last few IP addresses
IP_ARRAY=(${SUBNET_IP//./ })
SUBNET_IP="${IP_ARRAY[0]}.${IP_ARRAY[1]}.255.255"

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: metallb-system
  name: config
data:
  config: |
    address-pools:
    - name: default
      protocol: layer2
      addresses:
      - ${SUBNET_IP}/28
EOF

date
echo "Waiting for the deployments of the foundational configurations to be ready"
# 1200s = 20 min, to handle even a horrendously slow setup. Regular is 10 to 30 seconds.
kubectl wait --for=condition=Available --namespace local-path-storage deployment/local-path-provisioner --timeout=1200s
date

echo "Done! The cluster is ready."
