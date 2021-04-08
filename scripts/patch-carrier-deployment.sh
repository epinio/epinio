#!/bin/bash

# This script should be used while doing development on Carrier.
# When running `carrier install`, the Carrier Deployment will try to apply
# this file: embedded-files/carrier/server.yaml . This file assumes an image
# with the current binary has been built and pushed to Dockerhub. While developing
# though, we don't always build and push such an image so the deployment will fail.
# By setting CARRIER_DONT_WAIT_FOR_DEPLOYMENT we allow the installation to continue
# and by calling `make patch-carrier-deployment` (which calls this script), we
# fix the deployment as this:
# - We use an image that exists (the base image used when building the final image)
# - We create a PVC and a dummy pod that mounts that PVC.
# - We copy the built binary on the PVC by calling `kubectl cp` on the dummy pod.
# - We mount the same PVC on the carrier server deployment at the location where
#   the binary is expected.
# Patching the deployment forces the pod to restart with a now existing image
# and the correct binary is in place.

export CARRIER_BINARY_PATH=${CARRIER_BINARY_PATH:-'dist/carrier-linux-amd64'}

echo "Creating the PVC"
cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: carrier-binary
  namespace: carrier
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 100Mi
EOF

echo "Creating the dummy copier Pod"
cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: Pod
metadata:
  name: carrier-copier
  namespace: carrier
spec:
  volumes:
    - name: carrier-binary
      persistentVolumeClaim:
        claimName: carrier-binary
  containers:
    - name: copier
      image: busybox
      command: ["/bin/sh", "-ec", "while :; do printf '.'; sleep 5 ; done"]
      volumeMounts:
        - mountPath: "/carrier"
          name: carrier-binary
EOF

echo "Waiting for dummy pod to be ready"
kubectl wait --for=condition=ready pod -n carrier carrier-copier

echo "Copying the binary on the PVC"
kubectl cp ${CARRIER_BINARY_PATH} carrier/carrier-copier:/carrier/carrier

echo "Patching the carrier-server deployment to use the copied binary"
read -r -d '' PATCH <<EOF
{
  "spec": { "template": {
      "spec": {
        "volumes": [
          {
            "name":"carrier-binary",
            "persistentVolumeClaim": {
              "claimName": "carrier-binary"
            }
          }
        ],
        "containers": [{
          "name": "carrier-server",
          "image": "splatform/epinio-base:$(git describe --tags --abbrev=0)",
          "command": [
            "/carrier-binary/carrier",
            "server"
          ],
          "volumeMounts": [
            {
              "name": "carrier-binary",
              "mountPath": "/carrier-binary"
            }
          ]
        }]
      }
    }
  }
} 
EOF
kubectl patch deployment -n carrier carrier-server -p "${PATCH}"


echo "Ensuring the deployment is restarted"
kubectl rollout restart deployment -n carrier carrier-server

# https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands#-em-status-em-
echo "Waiting for the rollout of the deployment to complete"
kubectl rollout status deployment -n carrier carrier-server  --timeout=120s
