#!/bin/bash

set -e

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "${SCRIPT_DIR}/helpers.sh"

# UNAME should be darwin or linux
UNAME="$(uname | tr "[:upper:]" "[:lower:]")"

# EPINIO_BINARY is used to execute the installation commands
EPINIO_BINARY="./dist/epinio-$UNAME-amd64"

# IMAGE_TAG is the one built from the 'make build-images'
IMAGE_TAG="$(git describe --tags)"

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

function retry {
  retry=0
  maxRetries=$1
  retryInterval=$2
  local result
  until [ ${retry} -ge ${maxRetries} ]
  do
    echo -n "."
    result=$(eval "$3") && break
    retry=$[${retry}+1]
    sleep ${retryInterval}
  done

  if [ ${retry} -ge ${maxRetries} ]; then
    echo "Failed to run "$3" after ${maxRetries} attempts!"
    exit 1
  fi

  echo " ✔️"
}

function deploy_epinio_latest_released {
  helm repo add epinio https://epinio.github.io/helm-charts
  helm repo update
  helm upgrade --wait --install -n epinio --create-namespace epinio epinio/epinio \
       --set global.domain="$EPINIO_SYSTEM_DOMAIN" \
       --set dex.config.issuer="https://auth.$EPINIO_SYSTEM_DOMAIN" \
       --set "dex.ingress.tls[0].hosts[0]=auth.$EPINIO_SYSTEM_DOMAIN" \
       --set "dex.ingress.tls[0].secretName=dex-tls" \
       --set "dex.ingress.hosts[0].host=auth.$EPINIO_SYSTEM_DOMAIN" \
       --set "dex.ingress.hosts[0].paths[0].path=/" \
       --set "dex.ingress.hosts[0].paths[0].pathType=Prefix"
}

# Ensure we have a value for --system-domain
prepare_system_domain
# Check Dependencies
check_dependency kubectl helm
# Create docker registry image pull secret
create_docker_pull_secret

echo "Installing Epinio"
# Deploy epinio latest release to test upgrade
if [[ $EPINIO_RELEASED ]]; then
  echo "Deploying latest released epinio server image"
  deploy_epinio_latest_released
else
  echo "Importing locally built epinio server image"
  k3d image import -c epinio-acceptance "ghcr.io/epinio/epinio-server:${IMAGE_TAG}"

  helm upgrade --install --create-namespace -n epinio \
    --set global.domain="$EPINIO_SYSTEM_DOMAIN" \
    --set dex.config.issuer="https://auth.$EPINIO_SYSTEM_DOMAIN" \
    --set "dex.ingress.tls[0].hosts[0]=auth.$EPINIO_SYSTEM_DOMAIN" \
    --set "dex.ingress.tls[0].secretName=dex-tls" \
    --set "dex.ingress.hosts[0].host=auth.$EPINIO_SYSTEM_DOMAIN" \
    --set "dex.ingress.hosts[0].paths[0].path=/" \
    --set "dex.ingress.hosts[0].paths[0].pathType=Prefix" \
    --set image.epinio.tag="${IMAGE_TAG}" \
    epinio helm-charts/chart/epinio --wait "$@"

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
fi

# Check Epinio Installation
# Retry 5 times and sleep 1s because sometimes it takes a while before epinio server is ready

echo "-------------------------------------"
echo "Trying to login"

echo " - Creating the settings file"
SETTINGS_LOCATION=$(${EPINIO_BINARY} settings show | grep 'Settings:' | cut -d ' ' -f2)
rm $SETTINGS_LOCATION
${EPINIO_BINARY} settings update-ca >/dev/null

echo " - Updating access token"
ACCESS_TOKEN=$(./scripts/login.sh admin@epinio.io password https://auth.$EPINIO_SYSTEM_DOMAIN)
sed -i "s/accesstoken: \"\"/accesstoken: \"$ACCESS_TOKEN\"/" "$SETTINGS_LOCATION"

echo "-------------------------------------"
echo -n "Trying to getting info"
retry 5 2 "${EPINIO_BINARY} info"

# let's have at least a couple of success
echo -n "Trying to getting info again"
retry 5 2 "${EPINIO_BINARY} info"
echo "-------------------------------------"

${EPINIO_BINARY} info

echo
echo "Done preparing k3d environment!"
