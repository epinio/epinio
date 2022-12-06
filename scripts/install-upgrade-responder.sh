#!/bin/bash

set -e

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "$SCRIPT_DIR/helpers.sh"


# Ensure we have a value for domain
prepare_system_domain

echo "Installing upgrade-responder"
helm upgrade --install upgrade-responder --create-namespace -n epinio  \
  --set "applicationName=epinio" \
  --set "grafana.ingress.hosts[0]=grafana.$EPINIO_SYSTEM_DOMAIN" \
  "$SCRIPT_DIR/../helm-charts/chart/upgrade-responder" \
  --wait

kubectl patch secret --namespace epinio upgrade-responder-grafana -p='{"data":{"admin-password": "cGFzc3dvcmQ="}}' > /dev/null

echo "Restarting Grafana.."
kubectl delete pod -n epinio -l app.kubernetes.io/name=grafana > /dev/null

echo "https://grafana.$EPINIO_SYSTEM_DOMAIN"
