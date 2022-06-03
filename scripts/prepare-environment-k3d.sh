#!/bin/bash

set -e

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "${SCRIPT_DIR}/helpers.sh"

# UNAME should be darwin or linux
UNAME="$(uname | tr "[:upper:]" "[:lower:]")"

# EPINIO_BINARY is used to execute the installation commands
EPINIO_BINARY="./dist/epinio-$UNAME-amd64"

function check_dependency {
	for dep in "$@"
	do
		if ! [ -x "$(command -v $dep)" ]; then
			echo "Error: ${dep} is not installed." >&2
  			exit 1
		fi
	done

}

function create_docker_pull_secret {
	if [[ "$REGISTRY_USERNAME" != "" && "$REGISTRY_PASSWORD" != "" && ! $(kubectl get secret regcred > /dev/null 2>&1) ]];
	then
		kubectl create secret docker-registry regcred \
			--docker-server https://index.docker.io/v1/ \
			--docker-username $REGISTRY_USERNAME \
			--docker-password $REGISTRY_PASSWORD
	fi
}

# Ensure we have a value for --system-domain
prepare_system_domain
# Check Dependencies
check_dependency kubectl helm
# Create docker registry image pull secret
create_docker_pull_secret

echo "Installing Epinio"
helm upgrade --install --create-namespace -n epinio \
	--set global.domain="$EPINIO_SYSTEM_DOMAIN" \
	epinio helm-charts/chart/epinio --wait "$@"

echo "Importing locally built epinio server image"
k3d image import -c epinio-acceptance ghcr.io/epinio/epinio-server:latest

# compile coverage binary and add required env var
if [ -n "$EPINIO_COVERAGE" ]; then
  echo "Compiling coverage binary"
  GOARCH="amd64" GOOS="linux" CGO_ENABLED=0 go test -c -covermode=count -coverpkg ./...
  export EPINIO_BINARY_PATH="epinio.test"
  echo "Patching epinio for coverage env var"
  kubectl patch deployments -n epinio epinio-server --type=json \
    -p '[{"op": "add", "path": "/spec/template/spec/containers/0/env/-", "value": {"name": "EPINIO_COVERAGE", "value": "true"}}]'
fi

# Patch Epinio
./scripts/patch-epinio-deployment.sh

if [ -n "$EPINIO_COVERAGE" ]; then
  echo "Patching epinio ingress with coverage helper container"
  kubectl patch ingress -n epinio epinio --type=json \
    -p '[{"op": "add", "path": "/spec/rules/0/http/paths/-", "value":
    { "backend": { "service": { "name": "epinio-server", "port": { "number": 80 } } }, "path": "/exit", "pathType": "ImplementationSpecific" } }]'
fi

# Check Epinio Installation
# Retry 5 times because sometimes it takes a while before epinio server
# is ready after patching.
retry=0
maxRetries=5
retryInterval=1
until [ ${retry} -ge ${maxRetries} ]
do
	"${EPINIO_BINARY}" login -u admin -p password --trust-ca https://epinio.$EPINIO_SYSTEM_DOMAIN && break
	retry=$[${retry}+1]
	echo "Retrying [${retry}/${maxRetries}] in ${retryInterval}(s) "
	sleep ${retryInterval}
done

if [ ${retry} -ge ${maxRetries} ]; then
  echo "Failed to reach epinio endpoint after ${maxRetries} attempts!"
  exit 1
fi

"${EPINIO_BINARY}" info

echo "Done preparing k3d environment!"
