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

echo
echo "Preparing InfluxDB"

INFLUXDB_POD_NAME=$(kubectl get pod -n epinio -l app.kubernetes.io/name=influxdb -o jsonpath="{.items[0].metadata.name}")
# Setup the upgrade-responder Kubernetes Version query
QUERY='CREATE CONTINUOUS QUERY "cq_by_kubernetes_version_down_sampling" ON "epinio_upgrade_responder" BEGIN SELECT count("value") as total INTO "by_kubernetes_version_down_sampling" FROM "upgrade_request" GROUP BY time(5m),"kubernetes_version" END'
kubectl exec -n epinio $INFLUXDB_POD_NAME -- influx -execute "$QUERY"
# Setup the upgrade-responder Kubernetes Platform query
QUERY='CREATE CONTINUOUS QUERY "cq_by_kubernetes_platform_down_sampling" ON "epinio_upgrade_responder" BEGIN SELECT count("value") as total INTO "by_kubernetes_platform_down_sampling" FROM "upgrade_request" GROUP BY time(5m),"kubernetes_platform" END'
kubectl exec -n epinio $INFLUXDB_POD_NAME -- influx -execute "$QUERY"

# Changing Grafana password
echo "Preparing Grafana"

kubectl patch secret --namespace epinio upgrade-responder-grafana -p='{"data":{"admin-password": "cGFzc3dvcmQ="}}' > /dev/null
kubectl delete pod -n epinio -l app.kubernetes.io/name=grafana > /dev/null

echo "You can access your Grafana instance from 'https://grafana.$EPINIO_SYSTEM_DOMAIN' with admin/password"
