#!/bin/bash

set -e

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
	if [[ "$REGISTRY_USERNAME" != "" && "$REGISTRY_PASSWORD" != ""  ]]; 
	then
		kubectl create secret docker-registry regcred \
			--docker-server https://index.docker.io/v1/ \
			--docker-username $REGISTRY_USERNAME \
			--docker-password $REGISTRY_PASSWORD
	fi
}

function prepare_system_domain {
  if [[ -z "${EPINIO_SYSTEM_DOMAIN}" ]]; then
    echo "EPINIO_SYSTEM_DOMAIN not set. Trying to use a magic domain..."
    EPINIO_CLUSTER_IP=$(docker inspect k3d-epinio-acceptance-server-0 | jq -r '.[0]["NetworkSettings"]["Networks"]["epinio-acceptance"]["IPAddress"]')
    if [[ -z $EPINIO_CLUSTER_IP ]]; then
      echo "Couldn't find the cluster's IP address"
      exit 1
    fi

    export EPINIO_SYSTEM_DOMAIN="${EPINIO_CLUSTER_IP}.omg.howdoi.website"
  else
    echo "Using ${EPINIO_SYSTEM_DOMAIN} for --system-domain"
  fi
}

# Ensure we have a value for --system-domain
prepare_system_domain
# Check Dependencies
check_dependency kubectl helm
# Create docker registry image pull secret
create_docker_pull_secret

# Install Epinio
EPINIO_DONT_WAIT_FOR_DEPLOYMENT=1 "${EPINIO_BINARY}" install --system-domain "${EPINIO_SYSTEM_DOMAIN}"

# Patch Epinio
./scripts/patch-epinio-deployment.sh

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
