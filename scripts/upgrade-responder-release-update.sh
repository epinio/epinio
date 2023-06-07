#! /bin/bash
#
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

set -euo pipefail

K8S_NAMESPACE=${K8S_NAMESPACE}
K8S_DRY_RUN=${K8S_DRY_RUN:-none}
EKS_ENDPOINT=${EKS_ENDPOINT}

COGNITO_USERNAME=${COGNITO_USERNAME}
COGNITO_PASSWORD=${COGNITO_PASSWORD}
COGNITO_CLIENT_ID=${COGNITO_CLIENT_ID}

COMMITTER_TOKEN=${COMMITTER_TOKEN}

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "${SCRIPT_DIR}/cognito-login.sh"

# Fetch the releases and build the Upgrade Responder Response JSON config
# Ref: https://github.com/longhorn/upgrade-responder#response-json-config-example
function UpgradeResponderResponseJSON
{
  http_code=$(
    curl -s -o /tmp/epinio_releases.json -w '%{http_code}' \
    --header "Authorization: token ${COMMITTER_TOKEN}" \
      https://api.github.com/repos/epinio/epinio/releases;
  )

  if [[ $http_code -ne 200 ]]; then
    echo "Error calling the Github APIs. Code: $http_code" > /dev/stderr
    echo "Response: $(cat /tmp/epinio_releases.json)" > /dev/stderr
    exit 1
  fi

  cat /tmp/epinio_releases.json | \
  jq '.[] | select(.draft | not) | {
    Name: (.name | split(" ")[0]),
    ReleaseDate: .published_at,
    MinUpgradableVersion: "",
    Tags: [ .tag_name ],
    ExtraInfo: null
  }' | \
  jq -n '. |= [inputs]' | \
  jq '(first | .Tags) |= .+ ["latest"] | { 
    versions: .,
    requestIntervalInMinutes: 60
  }'
}

UPGRADE_RESPONDER_RESPONSE_JSON=$(UpgradeResponderResponseJSON)

echo "Updating the Upgrade Responder Response JSON with the latest Epinio release:"
echo ${UPGRADE_RESPONDER_RESPONSE_JSON} | jq .

# Cleanup the JSON removing the spaces before updating the ConfigMap
UPGRADE_RESPONDER_RESPONSE_JSON=$(echo ${UPGRADE_RESPONDER_RESPONSE_JSON} | jq -c .)

kubectl get configmap configmap-upgrade-responder --namespace ${K8S_NAMESPACE} \
    --context epinio.version.rancher.io -o json | \
    jq --arg add ${UPGRADE_RESPONDER_RESPONSE_JSON} '.data["upgrade-responder-config.json"] = $add' | \
    kubectl apply --dry-run=${K8S_DRY_RUN} --context epinio.version.rancher.io -f -
