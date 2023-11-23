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

timeout=480s

# This script should be used while doing development on Epinio.
# When installing epinio, Helm will try to apply this file:
# assets/embedded-files/epinio/server.yaml . This file assumes an image
# with the current binary has been built and pushed to Dockerhub. While developing
# though, we don't always build and push such an image so the deployment will fail.
# By calling `make patch-epinio-deployment` (which calls this script), we
# fix the deployment as this:
# - We use an image that exists (the base image used when building the final image)
# - We create a PVC and a dummy pod that mounts that PVC.
# - We copy the built binary on the PVC by calling `kubectl cp` on the dummy pod.
# - We mount the same PVC on the epinio server deployment at the location where
#   the binary is expected.
# Patching the deployment forces the pod to restart with a now existing image
# and the correct binary is in place.

export EPINIO_BINARY_PATH="${EPINIO_BINARY_PATH:-dist/epinio-linux-amd64}"
export EPINIO_BINARY_TAG="${EPINIO_BINARY_TAG:-$(git describe --tags)}"

echo
echo Configuration
echo "  - Binary: ${EPINIO_BINARY_PATH}"
echo "  - Tag:    ${EPINIO_BINARY_TAG}"
echo

if [ ! -f "$EPINIO_BINARY_PATH" ]; then
  echo "epinio binary path is not a file"
  exit 1
fi

if [ -z "$EPINIO_BINARY_TAG" ]; then
  echo "epinio binary tag is empty"
  exit 1
fi

echo "Creating the PVC"
cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: epinio-binary
  namespace: epinio
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 150Mi
EOF

echo "Creating the dummy copier Pod"
cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: Pod
metadata:
  name: epinio-copier
  namespace: epinio
  annotations:
    linkerd.io/inject: disabled
spec:
  affinity:
    podAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchExpressions:
          - key: app.kubernetes.io/component
            operator: In
            values:
            - epinio-server
        topologyKey: kubernetes.io/hostname
  volumes:
    - name: epinio-binary
      persistentVolumeClaim:
        claimName: epinio-binary
  containers:
    - name: copier
      image: busybox:stable
      command: ["/bin/sh", "-ec", "trap : TERM INT; sleep infinity & wait"]
      volumeMounts:
        - mountPath: "/epinio"
          name: epinio-binary
EOF

echo "Waiting for dummy pod to be ready"
kubectl wait --for=condition=ready --timeout=$timeout pod -n epinio epinio-copier

echo "Copying the binary on the PVC"
# Notes
# 1. kubectl cp breaks because of the colon in `pod:path`. Thus the more complex tar construction.
# 2. Cannot use absolute paths, i.e. `/foo`. This is a disk-relative path on Windows, and gets
#    expanded weirdly by the kubectl commands.
#    Relative paths are ok, as the default CWD in the container is the root (`/`).
#    I.e. `foo` is `/foo` pod-side.

( cd       "$(dirname  "${EPINIO_BINARY_PATH}")"
  tar cf - "$(basename "${EPINIO_BINARY_PATH}")"
) | kubectl exec -i -n epinio -c copier epinio-copier -- tar xf -
kubectl exec -i -n epinio -c copier epinio-copier -- mv "$(basename "${EPINIO_BINARY_PATH}")" epinio/epinio
kubectl exec -i -n epinio -c copier epinio-copier -- chmod ugo+x epinio/epinio
kubectl exec -i -n epinio -c copier epinio-copier -- ls -l epinio

echo "Deleting the epinio-copier to avoid multi-attach issue between pods"
kubectl delete pod -n epinio epinio-copier

# On Windows the shasum command is not in the path
[[ $(uname -s) =~ "MINGW64_NT" ]] && SHASUM=/bin/core_perl/shasum

# https://stackoverflow.com/a/5773761
EPINIO_BINARY_HASH=($(${SHASUM:=shasum} ${EPINIO_BINARY_PATH}))

helper=""
if [ -n "$GOCOVERDIR" ]; then
  helper=',{"name": "tools", "image": "alpine", "command": ["/bin/sh", "-ec", "trap : TERM INT; sleep infinity & wait"], "volumeMounts": [{"mountPath": "/tmp", "name": "tmp-volume"}]}'
fi

# Due to PV Multi-Attach Error on AKS the RollingUpdate strategy is needed.
# It will remove the original epinio-server pod immediatelly after patching.
# Note: Scaling deployment to 0 replicas and then back to 1 after patching would work similarly.
# Ref. https://github.com/andyzhangx/demo/blob/master/issues/azuredisk-issues.md#25-multi-attach-error
echo "Patching the epinio-server deployment to use the copied binary"
PATCH=$(cat <<EOF
{ "spec": { "template": {
      "metadata": {
        "annotations": {
          "binary-hash": "${EPINIO_BINARY_HASH}"
        }
      },
      "spec": {
        "volumes": [
        {
          "name":"epinio-binary",
          "persistentVolumeClaim": {
            "claimName": "epinio-binary"
          }
        }],
        "containers": [{
          "name": "epinio-server",
          "image": "ghcr.io/epinio/epinio-server:latest",
          "command": [
            "/epinio-binary/epinio",
            "server"
          ],
          "volumeMounts": [
            {
              "name": "epinio-binary",
              "mountPath": "/epinio-binary"
            }
          ]
        }$helper]
      }
    },
    "strategy": {
      "rollingUpdate": {
        "maxSurge": 0,
        "maxUnavailable": 1
      },
      "type": "RollingUpdate"
    }
  }
}
EOF
)

kubectl patch deployment -n epinio epinio-server -p "${PATCH}"

# https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands#-em-status-em-
echo "Waiting for the rollout of the deployment to complete"
kubectl rollout status deployment -n epinio epinio-server  --timeout=$timeout
