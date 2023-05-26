#!/bin/sh
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

set -ex

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

kubectl exec -n epinio pod/"$name" . -c tools -- sh -c "ls /tmp/cov*" | \
    xargs -n1 basename | \
    xargs -I{} kubectl cp epinio/"$name":tmp/{} /tmp/{} -c tools 2>/dev/null

go tool covdata textfmt -i=/tmp -o coverprofile.out

