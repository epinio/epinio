#!/bin/bash

set -e

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "${SCRIPT_DIR}/helpers.sh"

# UNAME should be darwin or linux
UNAME="$(uname | tr "[:upper:]" "[:lower:]")"

# EPINIO_BINARY is used to execute the installation commands
EPINIO_BINARY="./dist/epinio-"${UNAME}"-amd64"

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

helm repo add cert-manager https://charts.jetstack.io
helm repo update

echo "Installing Cert Manager"
helm upgrade --install cert-manager --create-namespace -n cert-manager \
  --set installCRDs=true \
	--set enable-certificate-owner-ref=true \
	cert-manager/cert-manager --version 1.7.1 \
	--wait

echo "Installing Epinio"
helm upgrade --install --create-namespace -n epinio \
	--set global.domain="$EPINIO_SYSTEM_DOMAIN" \
	epinio helm-charts/chart/epinio --wait

echo "Importing locally built epinio server image"
k3d image import -c epinio-acceptance ghcr.io/epinio/epinio-server:latest

# Patch Epinio
./scripts/patch-epinio-deployment.sh

"${EPINIO_BINARY}" config update

# Check Epinio Installation
# Retry 5 times because sometimes it takes a while before epinio server
# is ready after patching.
retry=0
maxRetries=5
retryInterval=1
until [ ${retry} -ge ${maxRetries} ]
do
	${EPINIO_BINARY} info && break
	retry=$[${retry}+1]
	echo "Retrying [${retry}/${maxRetries}] in ${retryInterval}(s) "
	sleep ${retryInterval}
done

if [ ${retry} -ge ${maxRetries} ]; then
  echo "Failed to reach epinio endpoint after ${maxRetries} attempts!"
  exit 1
fi

echo "Done preparing k3d environment!"
