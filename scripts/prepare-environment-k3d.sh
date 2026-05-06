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

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "${SCRIPT_DIR}/helpers.sh"

# UNAME should be darwin or linux
UNAME="$(uname | tr "[:upper:]" "[:lower:]")"

# EPINIO_BINARY is used to execute the installation commands
EPINIO_BINARY="./dist/epinio-$UNAME-amd64"

# IMAGE_TAG is the one built from the 'make build-images'
IMAGE_TAG="test-$(git describe --tags)"

function check_dependency {
  echo "Check dependencies"
	for dep in "$@"
	do
		if ! [ -x "$(command -v $dep)" ]; then
			echo "Error: ${dep} is not installed." >&2
  			exit 1
		fi
	done

}

function create_docker_pull_secret {
  echo "Check for docker pull secret"
  if [[ -n "$REGISTRY_USERNAME" && -n "$REGISTRY_PASSWORD" ]]; then
    if ! kubectl get secret regcred > /dev/null 2>&1; then
      echo "Creating docker pull secret"
      kubectl create secret docker-registry regcred \
        --docker-server https://index.docker.io/v1/ \
        --docker-username $REGISTRY_USERNAME \
        --docker-password $REGISTRY_PASSWORD
    fi
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
  echo "using epinio system domain and port: ${EPINIO_DOMAIN_AND_PORT}"
  helm upgrade --wait --install -n epinio --create-namespace epinio epinio/epinio \
    --set global.domain="${EPINIO_SYSTEM_DOMAIN}" \
    --set dex.issuer.port="${EPINIO_PORT}" \
    --set "extraEnv[0].name=KUBE_API_QPS" --set-string "extraEnv[0].value=50" \
    --set "extraEnv[1].name=KUBE_API_BURST" --set-string "extraEnv[1].value=100" \
    --set server.disableTracking="true" \
    --set ingress.nginxSSLRedirect="false" \
    --set dex.ui.redirectURI="https://epinio.${EPINIO_DOMAIN_AND_PORT}/auth/verify"
}

# Ensure we have a value for --system-domain
prepare_system_domain
# Check Dependencies
check_dependency kubectl helm
# Create docker registry image pull secret
create_docker_pull_secret

echo "Installing nginx"

helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx

helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
		--namespace ingress-nginx \
		--create-namespace \
		--set controller.ingressClassResource.default=true \
    --set controller.admissionWebhooks.enabled=false

echo "Installing Epinio"
# Deploy epinio latest release to test upgrade
if [[ $EPINIO_RELEASED ]]; then
  echo "Deploying latest released epinio server image"
  deploy_epinio_latest_released
else
  # todo: check if k3d is being used and images actually need to be imported
  if [[ -z "${EPINIO_EXISTING_CLUSTER}" ]]; then
    echo "Importing locally built epinio server image"
    k3d image import --verbose -c epinio-acceptance "ghcr.io/epinio/epinio-server:${IMAGE_TAG}"
    echo "Importing locally built epinio unpacker image"
    k3d image import --verbose -c epinio-acceptance "ghcr.io/epinio/epinio-unpacker:${IMAGE_TAG}"
    echo "Importing locally built images: Completed"
  fi

  echo "using epinio system domain and port: ${EPINIO_DOMAIN_AND_PORT}"
  helm upgrade --install --create-namespace -n epinio \
    --set global.domain="${EPINIO_SYSTEM_DOMAIN}" \
    --set dex.issuer.port="${EPINIO_PORT}" \
    --set image.epinio.tag="${IMAGE_TAG}" \
    --set image.bash.tag="${IMAGE_TAG}" \
    --set server.disableTracking="true" \
    --set "extraEnv[0].name=KUBE_API_QPS" --set-string "extraEnv[0].value=50" \
    --set "extraEnv[1].name=KUBE_API_BURST" --set-string "extraEnv[1].value=100" \
    --set ingress.nginxSSLRedirect="false" \
    --set dex.ui.redirectURI="https://epinio.${EPINIO_DOMAIN_AND_PORT}/auth/verify" \
    epinio helm-charts/chart/epinio --wait "$@"

  # compile coverage binary and add required env var
  if [ -n "$GOCOVERDIR" ]; then
    echo "Compiling coverage binary"
    GOARCH="amd64" GOOS="linux" CGO_ENABLED=0 go build -cover -covermode=count -coverpkg ./... -o $EPINIO_BINARY
    echo "Patching epinio for coverage env var"
    kubectl patch deployments -n epinio epinio-server --type=json \
      -p '[{"op": "add", "path": "/spec/template/spec/containers/0/env/-", "value": {"name": "GOCOVERDIR", "value": "/tmp"}}]'
  fi

  # Patch Epinio
  ./scripts/patch-epinio-deployment.sh

  if [ -n "$GOCOVERDIR" ]; then
    echo "Patching epinio ingress with coverage helper container"
    kubectl patch ingress -n epinio epinio --type=json \
      -p '[{"op": "add", "path": "/spec/rules/0/http/paths/-", "value":
      { "backend": { "service": { "name": "epinio-server", "port": { "number": 80 } } }, "path": "/exit", "pathType": "ImplementationSpecific" } }]'
  fi
fi

# Increase ingress timeouts so slower namespace operations in CI don't hit nginx 504s.
echo "Configuring Epinio ingress timeouts"
kubectl annotate ingress -n epinio epinio \
  nginx.ingress.kubernetes.io/proxy-read-timeout="600" \
  nginx.ingress.kubernetes.io/proxy-send-timeout="600" \
  nginx.ingress.kubernetes.io/proxy-connect-timeout="120" \
  --overwrite || true

echo "-------------------------------------"
echo "Check for binary"
${EPINIO_BINARY} version
echo "-------------------------------------"
echo "Cleanup old settings"
rm -f $HOME/.config/epinio/settings.yaml

# Check Epinio Installation
# Retry 5 times and sleep 1s because sometimes it takes a while before epinio server is ready

echo "-------------------------------------"
echo -n "Trying to login using:"
echo -n "${EPINIO_BINARY} login -u admin -p password --trust-ca https://epinio.$EPINIO_DOMAIN_AND_PORT"
retry 5 10 "${EPINIO_BINARY} login -u admin -p password --trust-ca https://epinio.$EPINIO_DOMAIN_AND_PORT"
echo "-------------------------------------"
echo -n "Trying to getting info"
retry 5 10 "${EPINIO_BINARY} info"
echo "-------------------------------------"

# Check no tls-dex cert conflict issue
# Counting logs of undesired message
message_dex_tls="unexpected managed Secret Owner Reference field on Secret --enable-certificate-owner-ref=true"
check_dex_log_count=$(kubectl logs  -n cert-manager -lapp=cert-manager --tail=-1 | grep "${message_dex_tls}"  | wc -l)

# Exiting with count of bad logs if more than 10 are found
if [ $check_dex_log_count -gt 10 ]; then
 echo
 echo "-------------------------------------"
 echo "Warning: 'dex-tls' secrets may be be updated many times a second."
 echo "More than '${check_dex_log_count}' logs found in 'cert-manager/cert-manager' pod with entry = '${message_dex_tls}'"
 echo "Exiting installation"
 echo "-------------------------------------" 
 exit 1
fi

${EPINIO_BINARY} info

echo
echo "Done preparing k3d environment!"
