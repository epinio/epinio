#!/bin/bash

set -e

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "$SCRIPT_DIR/helpers.sh"


# Ensure we have a value for domain
prepare_system_domain


# Get the latest Epinio versions, update the Upgrade Responder config map and restart
curl -s https://api.github.com/repos/epinio/epinio/releases | \
  jq '.[] | {
    Name: (.name | split(" ")[0]),
    ReleaseDate: .published_at,
    MinUpgradableVersion: "",
    Tags: [ .tag_name ],
    ExtraInfo: null
  }' | \
  jq -n '. |= [inputs]' | \
  jq '(first | .Tags) |= .+ ["latest"] | { 
    versions: .,
    requestIntervalInMinutes: 5
  }' > upgrade-responder-config.json

UPGRADE_RESPONDER_CONFIG=$(jq -c . upgrade-responder-config.json)

cat > upgrade-responder-values.yaml <<YAML
applicationName: epinio

grafana:
  ingress:
    hosts:
    - grafana.${EPINIO_SYSTEM_DOMAIN}

configMap:
  responseConfig: |
    ${UPGRADE_RESPONDER_CONFIG}
YAML

echo "Installing upgrade-responder"
helm upgrade --install upgrade-responder --create-namespace -n epinio  \
  --values upgrade-responder-values.yaml \
  "$SCRIPT_DIR/../helm-charts/chart/upgrade-responder" \
  --wait

rm upgrade-responder-config.json upgrade-responder-values.yaml

echo
echo "Preparing InfluxDB"

INFLUXDB_POD_NAME=$(kubectl get pod -n epinio -l app.kubernetes.io/name=influxdb -o jsonpath="{.items[0].metadata.name}")

# Setup the upgrade-responder Epinio Server Version query
QUERY='CREATE CONTINUOUS QUERY "cq_by_epinio_server_version_down_sampling" ON "epinio_upgrade_responder" BEGIN SELECT count("value") as total INTO "by_epinio_server_version_down_sampling" FROM "upgrade_request" GROUP BY time(5m),"epinio_server_version" END'
kubectl exec -n epinio $INFLUXDB_POD_NAME -- influx -execute "$QUERY"
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

# Patching Epinio deployment
echo "Patching Epinio deployment"

kubectl patch deployments -n epinio epinio-server --type=json --patch \
'[
  {"op": "add", "path": "/spec/template/spec/containers/0/env/-", "value": {"name": "UPGRADE_RESPONDER_ADDRESS", "value": "http://upgrade-responder:8314/v1/checkupgrade"}},
  {"op": "add", "path": "/spec/template/spec/containers/0/env/-", "value": {"name": "DISABLE_TRACKING", "value": "false"}}
]'
