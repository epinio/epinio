#!/bin/bash

set -e

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "${SCRIPT_DIR}/helpers.sh"

# Ensure we have a value for --system-domain
prepare_system_domain

kubectl patch deployment -n epinio epinio-server \
  --patch '{"spec": {"template": {"spec": {"containers": [{"name": "epinio-server","env": [{"name":"ACCESS_CONTROL_ALLOW_ORIGIN", "value":"*"}]}]}}}}'

echo
echo "##################################################"
echo "#                                                #"
echo -e "#   Serving API docs at: \e[32mhttp://localhost:8080\e[0m   #"
echo "#                                                #"
echo "##################################################"
echo

SWAGGER_JSON_URL=https://epinio.$EPINIO_SYSTEM_DOMAIN/api/swagger.json
docker run -p 8080:8080 -e SWAGGER_JSON_URL=$SWAGGER_JSON_URL swaggerapi/swagger-ui
