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

# This script can be used to remove a Github Action Runner on an openSUSE or SLE
# distro. It will unconfigure the service and unregister the runner.
# Copy the script to runner:/home/<user> and run it as <user>.
# It requires GITHUB_REPOSITORY_URL (https) and GITHUB_RUNNER_TOKEN to be set
# e.g. export GITHUB_REPOSITORY_URL=https://github.com/epinio/epinio
# and  export GITHUB_RUNNER_TOKEN=<current token from github settings/actions/runners/new>
# Note: You can use the same token to add or remove multiple runners,
#       while it will expire after 1h.

set -e

if [ -z "$GITHUB_REPOSITORY_URL" ] || [ -z "$GITHUB_RUNNER_TOKEN" ]; then
  echo "Script requires GITHUB_REPOSITORY_URL and GITHUB_RUNNER_TOKEN to be set. Exiting"
  exit 1
fi

REPOSITORY_NAME=$(echo "$GITHUB_REPOSITORY_URL" | cut -d '/' -f 4- | sed -e 's|/$||' -e 's|/|-|g')
ACTIONS_RUNNER_SERVICE=actions.runner."$REPOSITORY_NAME".`hostname`.service

cd actions-runner
sudo systemctl stop "$ACTIONS_RUNNER_SERVICE"
sudo ./svc.sh uninstall
./config.sh remove --token "$GITHUB_RUNNER_TOKEN"
