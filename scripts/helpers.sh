#!/bin/bash
# Copyright © 2021 - 2023 SUSE LLC
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

function prepare_system_domain {
  echo "Prepare system domain"
  if [[ -z "${EPINIO_SYSTEM_DOMAIN}" ]]; then
    echo -e "\e[32mEPINIO_SYSTEM_DOMAIN not set. Trying to use a magic domain...\e[0m"
    EPINIO_CLUSTER_IP=$(docker inspect k3d-epinio-acceptance-server-0 | jq -r '.[0]["NetworkSettings"]["Networks"]["epinio-acceptance"]["IPAddress"]')
    if [[ -z $EPINIO_CLUSTER_IP ]]; then
      echo "Couldn't find the cluster's IP address"
      exit 1
    fi

    export EPINIO_SYSTEM_DOMAIN="${EPINIO_CLUSTER_IP}.sslip.io"
  fi
  echo -e "Using \e[32m${EPINIO_SYSTEM_DOMAIN}\e[0m for Epinio domain"
}

