#! /bin/sh
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

set -o nounset
set -o errexit

#########################
# External Dependencies #
#########################

${PROG_WHICH:="which"} \
  ${PROG_CURL:="curl"} \
  ${PROG_JQ:="jq"} \
  ${PROG_KUBECTL:="kubectl"} \
  ${PROG_WHICH} 1>/dev/null

##################################
# Configuration Helper Functions #
##################################

function ExtractRegionFromEndpoint
{
  local endpoint=${1}
  endpoint=${endpoint##*://}
  endpoint=${endpoint%.eks.amazonaws.com}
  endpoint=${endpoint##*.}
  echo "${endpoint}"
}

#################
# Configuration #
#################

: ${COGNITO_USERNAME:=""}
: ${COGNITO_PASSWORD:=""}
:
: ${COGNITO_CLIENT_ID:=""}
: ${EKS_ENDPOINT:=""}
:
: ${COGNITO_REGION:=$(ExtractRegionFromEndpoint "${EKS_ENDPOINT}")}
: ${COGNITO_ENDPOINT:="https://cognito-idp.${COGNITO_REGION}.amazonaws.com"}
:
: ${K8S_CLUSTER:="version.rancher.io"}
: ${K8S_NAMESPACE:=""}
:
: ${K8S_CONTEXT:="${K8S_NAMESPACE}.${K8S_CLUSTER}"}
: ${K8S_USER:="${COGNITO_USERNAME}@${COGNITO_ENDPOINT##*://}"}
:
: ${K8S_ROOT_CA_URL:="${EKS_ENDPOINT}/api/v1/namespaces/${K8S_NAMESPACE}/configmaps/kube-root-ca.crt"}

function AuthRequestPayload
{
  ${PROG_JQ} -e -r \
    --arg COGNITO_CLIENT_ID "${COGNITO_CLIENT_ID}" \
    --arg COGNITO_USERNAME "${COGNITO_USERNAME}" \
    --arg COGNITO_PASSWORD "${COGNITO_PASSWORD}" \
    '.
    | .ClientId=$ARGS.named.COGNITO_CLIENT_ID
    | .AuthParameters.USERNAME=$ARGS.named.COGNITO_USERNAME
    | .AuthParameters.PASSWORD=$ARGS.named.COGNITO_PASSWORD' \
    <<<'{ "AuthFlow": "USER_PASSWORD_AUTH", "AuthParameters": {} }'
}

function AuthHeader
{
  "${PROG_JQ}" -e -r \
    '.AuthenticationResult.IdToken
    | ["Authorization: Bearer", .]
    | join(" ")' \
  <<<"${*}"
}

function InitiateAuth
{
  ${PROG_CURL} \
    --fail \
    --silent \
    --header "X-Amz-Target: AWSCognitoIdentityProviderService.InitiateAuth" \
    --header "Content-Type: application/x-amz-json-1.1" \
    --request POST \
    --data "$(AuthRequestPayload)" \
    "${COGNITO_ENDPOINT}"
}

function AuthProviderParameters
{
  ${PROG_JQ} -e -r \
    --arg COGNITO_CLIENT_ID "${COGNITO_CLIENT_ID}" \
    '.AuthenticationResult
    | {
      "client-id": ($ARGS.named.COGNITO_CLIENT_ID),
      "refresh-token": (.RefreshToken),
      "id-token": (.IdToken),
      "idp-issuer-url": (.IdToken | split(".")[1] | @base64d | fromjson | .iss),
    }
    | to_entries
    | map(["--auth-provider-arg", .key, .value] | join("="))
    | .[]' \
    <<<"${*}"
}

function ClusterCertificate
{
  ${PROG_CURL} \
    --insecure \
    --fail \
    --silent \
    --header "$(AuthHeader $(InitiateAuth))" \
    ${K8S_ROOT_CA_URL} \
    | ${PROG_JQ} -e -r '.data["ca.crt"]'
}

function WriteUserToKubeConfig
{
  ${PROG_KUBECTL} config set-credentials "${K8S_USER}" \
    --auth-provider=oidc \
    $(AuthProviderParameters $(InitiateAuth))
}

function WriteClusterToKubeConfig
{
  ${PROG_KUBECTL} config set-cluster "${K8S_CLUSTER}" \
    --server="${EKS_ENDPOINT}" \
    --embed-certs=true \
    --certificate-authority=/dev/stdin <<<"$(ClusterCertificate)"
}

function WriteContextToKubeConfig
{
  ${PROG_KUBECTL} config set-context "${K8S_CONTEXT}" \
    --cluster="${K8S_CLUSTER}" \
    --user="${K8S_USER}" \
    --namespace="${K8S_NAMESPACE}"
}

function WriteKubeConfig
{
  WriteUserToKubeConfig
  WriteClusterToKubeConfig
  WriteContextToKubeConfig
}

if (( ${#} ))
then eval "${@}"
else WriteKubeConfig
fi
