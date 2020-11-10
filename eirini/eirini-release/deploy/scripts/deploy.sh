#!/bin/bash

set -euo pipefail

RED=1
GREEN=2
BLUE=4

print_message() {
  message=$1
  colour=$2
  printf "\\r\\033[00;3%sm%s\\033[0m\\n" "$colour" "$message"
}

warning=$(
  cat <<EOF
** WARNING **

This an example script used to create a standalone Eirini deployment.
It is used internally for testing, but is not supported for external use.

EOF
)

print_message "$warning" "$BLUE"

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

ns_directory="single-namespace"
if [ "${USE_MULTI_NAMESPACE:-true}" == "true" ]; then
  ns_directory="multi-namespace"
fi

export KUBECONFIG
KUBECONFIG=${KUBECONFIG:-$HOME/.kube/config}
KUBECONFIG=$(readlink -f "$KUBECONFIG")

export GOOGLE_APPLICATION_CREDENTIALS
GOOGLE_APPLICATION_CREDENTIALS=${GOOGLE_APPLICATION_CREDENTIALS:-""}
if [[ -n $GOOGLE_APPLICATION_CREDENTIALS ]]; then
  GOOGLE_APPLICATION_CREDENTIALS=$(readlink -f "$GOOGLE_APPLICATION_CREDENTIALS")
fi

cat "$PROJECT_ROOT"/deploy/**/namespace.yml | kubectl apply -f -

kubectl apply -f "$PROJECT_ROOT/deploy/core/"
kubectl apply -f "$PROJECT_ROOT/deploy/core/$ns_directory"
kubectl apply -f "$PROJECT_ROOT/deploy/workloads/"
kubectl apply -f "$PROJECT_ROOT/deploy/events/"
kubectl apply -f "$PROJECT_ROOT/deploy/events/$ns_directory"
kubectl apply -f "$PROJECT_ROOT/deploy/metrics/"
kubectl apply -f "$PROJECT_ROOT/deploy/metrics/$ns_directory"

# Install wiremock to mock the cloud controller
kubectl apply -f "$PROJECT_ROOT/deploy/testing/cc-wiremock"

pushd "$PROJECT_ROOT/deploy/scripts"
{
  ./generate_eirini_tls.sh "eirini-api.eirini-core.svc.cluster.local"
}
popd

deployments="$(kubectl get deployments \
  --namespace eirini-core \
  --template '{{range .items}}{{.metadata.name}}{{"\n"}}{{ end }}')"

for dep in $deployments; do
  kubectl rollout status deployment "$dep" --namespace eirini-core
done
