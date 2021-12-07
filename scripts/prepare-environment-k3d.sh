#!/bin/bash

set -e

# UNAME should be darwin or linux
UNAME="$(uname | tr "[:upper:]" "[:lower:]")"

# EPINIO_BINARY is used to execute the installation commands
EPINIO_BINARY="./dist/epinio-"${UNAME}"-amd64"

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

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
	if [[ "$REGISTRY_USERNAME" != "" && "$REGISTRY_PASSWORD" != ""  ]] && ! $(kubectl get secret regcred 2>&1 > /dev/null);
	then
		kubectl create secret docker-registry regcred \
			--docker-server https://index.docker.io/v1/ \
			--docker-username $REGISTRY_USERNAME \
			--docker-password $REGISTRY_PASSWORD
	fi
}

function prepare_system_domain {
  if [[ -z "${EPINIO_SYSTEM_DOMAIN}" ]]; then
    echo -e "\e[32mEPINIO_SYSTEM_DOMAIN not set. Trying to use a magic domain...\e[0m"
    EPINIO_CLUSTER_IP=$(docker inspect k3d-epinio-acceptance-server-0 | jq -r '.[0]["NetworkSettings"]["Networks"]["epinio-acceptance"]["IPAddress"]')
    if [[ -z $EPINIO_CLUSTER_IP ]]; then
      echo "Couldn't find the cluster's IP address"
      exit 1
    fi

    export EPINIO_SYSTEM_DOMAIN="${EPINIO_CLUSTER_IP}.omg.howdoi.website"
  fi
  echo -e "Using \e[32m${EPINIO_SYSTEM_DOMAIN}\e[0m for --system-domain"
}

# Ensure we have a value for --system-domain
prepare_system_domain
# Check Dependencies
check_dependency kubectl helm
# Create docker registry image pull secret
create_docker_pull_secret

echo "Preparing Epinio manifest"
echo "Replacing the system domain"
sed -i "s/10.86.4.38.omg.howdoi.website/$EPINIO_SYSTEM_DOMAIN/" installer/assets/examples/manifest.yaml

epinio_chart=$(readlink -e ${SCRIPT_DIR}/../epinio-helm-chart/chart/epinio)
registry_chart=$(readlink -e ${SCRIPT_DIR}/../epinio-helm-chart/chart/container-registry)

echo "Pointing to the local epinio helm chart"
sed -i "s|url: https://github.com/epinio/epinio-helm-chart/releases/download/epinio.*.tgz|path: ${epinio_chart}|" installer/assets/examples/manifest.yaml

echo "Pointing to the local container registry helm chart"
sed -i "s|path: assets/container-registry/chart/container-registry/|path: ${registry_chart}|" installer/assets/examples/manifest.yaml


echo "Installing Epinio"
pushd "${SCRIPT_DIR}/../installer" > /dev/null
../output/bin/epinio_installer install -m assets/examples/manifest.yaml
popd

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
