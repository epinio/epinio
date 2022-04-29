#!/bin/sh

set -e

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "${SCRIPT_DIR}/helpers.sh"

# Ensure we have a value for --system-domain
prepare_system_domain

# graceful exit for server
curl -vk https://epinio."$EPINIO_SYSTEM_DOMAIN"/exit

# wait for restart and get name
kubectl rollout status deployment -n epinio epinio-server

# copy server's coverprofile from helper container
name=$(kubectl get pods -n epinio -l app.kubernetes.io/name=epinio-server -o jsonpath="{.items[0].metadata.name}")
kubectl cp epinio/"$name":/tmp/coverprofile.out coverprofile.out -c tools
