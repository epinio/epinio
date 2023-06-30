#!/bin/bash

set -e

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "$SCRIPT_DIR/helpers.sh"

helm repo add rancher-latest https://releases.rancher.com/server-charts/latest
helm repo update

# Ensure we have a value for domain
prepare_system_domain

## Install Rancher

helm upgrade --install rancher rancher-latest/rancher \
  --namespace cattle-system --create-namespace \
  --set global.cattle.psp.enabled=false \
  --set "hostname=$EPINIO_SYSTEM_DOMAIN" \
  --set bootstrapPassword=password \
  --wait

# Wait for rancher deployment to be ready
kubectl rollout status deployment rancher -n cattle-system --timeout=300s
