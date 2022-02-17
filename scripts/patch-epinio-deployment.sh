#!/bin/bash

set -e

timeout=480s

# This script should be used while doing development on Epinio.
# When running `epinio install`, the Epinio Deployment will try to apply
# this file: assets/embedded-files/epinio/server.yaml . This file assumes an image
# with the current binary has been built and pushed to Dockerhub. While developing
# though, we don't always build and push such an image so the deployment will fail.
# By setting EPINIO_DONT_WAIT_FOR_DEPLOYMENT we allow the installation to continue
# and by calling `make patch-epinio-deployment` (which calls this script), we
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

echo "Import latest docker container in k3d"
k3d image import -c epinio-acceptance ghcr.io/epinio/epinio-server:${EPINIO_BINARY_TAG}

# On Windows the shasum command is not in the path
[[ $(uname -s) =~ "MINGW64_NT" ]] && SHASUM=/bin/core_perl/shasum

# https://stackoverflow.com/a/5773761
EPINIO_BINARY_HASH=($(${SHASUM:=shasum} ${EPINIO_BINARY_PATH}))

echo "Patching the epinio-server deployment to use the latest container"
PATCH=$(cat <<EOF
{ "spec": { "template": {
      "metadata": {
        "annotations": {
          "binary-hash": "${EPINIO_BINARY_HASH}",
          "kubectl.kubernetes.io/restartedAt": "$(date)"
        }
      },
      "spec": {
        "containers": [{
          "name": "epinio-server",
          "image": "ghcr.io/epinio/epinio-server:${EPINIO_BINARY_TAG}"
        }]
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
