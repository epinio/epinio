#!/bin/bash

set -e

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
  echo -e "Using \e[32m${EPINIO_SYSTEM_DOMAIN}\e[0m for Epinio domain"
}

