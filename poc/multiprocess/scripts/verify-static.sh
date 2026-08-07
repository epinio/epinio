#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
chart="$repo_root/poc/multiprocess/chart"
values="$chart/ci/test-values.yaml"
output_dir=${1:-$(mktemp -d)}
mkdir -p "$output_dir"

prebuilt="$output_dir/prebuilt.yaml"
staged="$output_dir/staged.yaml"
zero="$output_dir/zero-replicas.yaml"

assert_contains() {
  local pattern=$1
  local file=$2
  if ! rg --quiet --multiline "$pattern" "$file"; then
    echo "ASSERTION FAILED: pattern [$pattern] not found in $file" >&2
    return 1
  fi
}

assert_count() {
  local expected=$1
  local pattern=$2
  local file=$3
  local actual
  actual=$(rg --count "$pattern" "$file" || true)
  if [[ "$actual" != "$expected" ]]; then
    echo "ASSERTION FAILED: expected $expected matches for [$pattern] in $file, got $actual" >&2
    return 1
  fi
}

cd "$repo_root"

echo "[go] focused packages"
go test \
  ./pkg/api/core/v1/models \
  ./internal/manifest \
  ./internal/application \
  ./internal/helm \
  ./internal/api/v1/application

echo "[helm] lint and render"
helm lint "$chart" -f "$values"
helm template audit "$chart" --namespace poc -f "$values" >"$prebuilt"
helm template audit "$chart" --namespace poc -f "$values" --set epinio.staged=true >"$staged"
helm template audit "$chart" --namespace poc -f "$values" --set epinio.processes.worker.replicas=0 >"$zero"

echo "[render] resource and semantic assertions"
assert_count 2 '^kind: Deployment$' "$prebuilt"
assert_count 1 '^kind: Service$' "$prebuilt"
assert_count 1 '^kind: Ingress$' "$prebuilt"
assert_count 1 '^kind: Certificate$' "$prebuilt"
assert_count 1 '^kind: CronJob$' "$prebuilt"
assert_count 1 '^kind: Job$' "$prebuilt"
assert_contains 'name: multiprocess-poc-web.*\n(?:.*\n){0,15}spec:\n  replicas: 2' "$prebuilt"
assert_contains 'name: multiprocess-poc-worker.*\n(?:.*\n){0,15}spec:\n  replicas: 3' "$prebuilt"
assert_contains 'name: multiprocess-poc-worker.*\n(?:.*\n){0,15}spec:\n  replicas: 0' "$zero"
assert_contains 'schedule: "\* \* \* \* \*"' "$prebuilt"
assert_contains 'name: multiprocess-poc-release-r1' "$prebuilt"
assert_contains 'helm.sh/hook: pre-install,pre-upgrade' "$prebuilt"
assert_contains 'secretName: "multiprocess-poc-multiprocess-poc-tls"' "$prebuilt"
assert_contains 'image: docker.io/epinio-poc/multiprocess:v1.*\n        imagePullPolicy: IfNotPresent.*\n        command:\n          - python' "$prebuilt"
assert_count 4 '"/cnb/lifecycle/launcher"' "$staged"
assert_count 4 '^[[:space:]]+- "--"$' "$staged"

echo "[kubernetes] client-side schema dry runs"
kubectl create --dry-run=client -f "$prebuilt" -o name >"$output_dir/prebuilt-dry-run.txt"
kubectl create --dry-run=client -f "$staged" -o name >"$output_dir/staged-dry-run.txt"

echo "[git] whitespace check"
git diff --check

echo "PASS: focused Go tests, Helm lint/render assertions, Kubernetes dry runs, and diff check"
echo "Rendered evidence: $output_dir"
