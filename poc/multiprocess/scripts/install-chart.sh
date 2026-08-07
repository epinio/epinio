#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
namespace=${EPINIO_NAMESPACE:-epinio}
package_dir=$(mktemp -d)
trap 'rm -rf "$package_dir"' EXIT

cd "$repo_root"
helm package poc/multiprocess/chart --destination "$package_dir"
chart_archive="$package_dir/epinio-multiprocess-poc-0.2.1.tgz"

kubectl -n "$namespace" create configmap multiprocess-chart \
  --from-file=chart.tgz="$chart_archive" \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f poc/multiprocess/chart-server.yaml
kubectl rollout restart deployment/multiprocess-chart-server -n "$namespace"
kubectl rollout status deployment/multiprocess-chart-server -n "$namespace" --timeout=2m
kubectl apply -f poc/multiprocess/appchart.yaml

local_sha=$(sha256sum "$chart_archive" | awk '{print $1}')
cluster_sha=$(kubectl -n "$namespace" get configmap multiprocess-chart \
  -o jsonpath='{.binaryData.chart\.tgz}' | base64 -d | sha256sum | awk '{print $1}')

if [[ "$local_sha" != "$cluster_sha" ]]; then
  echo "ASSERTION FAILED: packaged chart and ConfigMap archive differ" >&2
  exit 1
fi

echo "Installed chart archive sha256=$cluster_sha"
kubectl -n "$namespace" get appchart multiprocess-poc \
  -o jsonpath='{.spec.helmChart}{"\n"}'
