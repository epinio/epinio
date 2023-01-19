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
