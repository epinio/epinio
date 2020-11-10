#!/bin/bash

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

for ns in $(ls -1 $PROJECT_ROOT/deploy/**/namespace.yml); do
  kubectl delete --wait=true -f "$ns" || true
done

# clean up remaining cluster-wide resources, if any
kubectl delete --recursive=true -f "$PROJECT_ROOT"/deploy || true
