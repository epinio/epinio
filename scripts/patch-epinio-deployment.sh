#!/bin/bash

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

#echo "Installing the EBS CSI driver:"
#kubectl apply -k "github.com/kubernetes-sigs/aws-ebs-csi-driver/deploy/kubernetes/overlays/stable/?ref=release-1.12"
#kubectl wait --for=condition=ready --timeout=$timeout deployment.apps/ebs-csi-controller -n kube-system || true # do not fail
echo "Installing the EBS CSI driver from helm chart:"
helm repo add aws-ebs-csi-driver https://kubernetes-sigs.github.io/aws-ebs-csi-driver
helm repo update
helm upgrade --install aws-ebs-csi-driver --namespace kube-system aws-ebs-csi-driver/aws-ebs-csi-driver --wait
sleep 120


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
      storage: 100Mi
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
sleep 100
echo "DEBUG check pv, pvc, sc here:"
kubectl get pv,pvc,sc -A
echo "DEBUG pvc:"
kubectl describe pvc -n epinio epinio-binary
echo "DEBUG list all pods to see provisioner is installed:"
kubectl get pods -A
echo "DEBUG epinio-copier logs here:"
kubectl logs -n epinio epinio-copier
echo "DEBUG epinio-copier describe here:"
kubectl describe pod -n epinio epinio-copier
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
if [ -n "$EPINIO_COVERAGE" ]; then
  helper=',{"name": "tools", "image": "alpine", "command": ["sleep","9000"], "volumeMounts": [{"mountPath": "/tmp", "name": "tmp-volume"}]}'
fi

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
    }
  }
}
EOF
)

kubectl patch deployment -n epinio epinio-server -p "${PATCH}"

# https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands#-em-status-em-
echo "Waiting for the rollout of the deployment to complete"
kubectl rollout status deployment -n epinio epinio-server  --timeout=$timeout
