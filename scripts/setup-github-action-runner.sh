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

# This script can be used to create a Github Action Runner on an openSUSE or SLE
# distro. It installs all the needed dependencies to run the acceptance tests
# and sets up docker and the runner as a service itself.
# Copy the script to runner:/home/<user> and run it as <user>.
# It requires GITHUB_REPOSITORY_URL (https) and GITHUB_RUNNER_TOKEN to be set
# e.g. export GITHUB_REPOSITORY_URL=https://github.com/epinio/epinio
# and  export GITHUB_RUNNER_TOKEN=<current token from github settings/actions/runners/new>
# Note: You can use the same token to add or remove multiple runners,
#       while it will expire after 1h.
# Optional export GITHUB_RUNNER_LABELS=<label1,label2> to automatically make the
# actions runner join/add "Labels"

set -e

if [ -z "$GITHUB_REPOSITORY_URL" ] || [ -z "$GITHUB_RUNNER_TOKEN" ]; then
  echo "Script requires GITHUB_REPOSITORY_URL and GITHUB_RUNNER_TOKEN to be set. Exiting"
  exit 1
fi

if [ -z "$GITHUB_RUNNER_LABELS" ]; then
  unset GITHUB_RUNNER_LABELS
 else
  runner_labels="--labels $GITHUB_RUNNER_LABELS"
fi

REPOSITORY_NAME=$(echo "$GITHUB_REPOSITORY_URL" | cut -d '/' -f 4- | sed -e 's|/$||' -e 's|/|-|g')
ACTIONS_RUNNER_SERVICE=actions.runner."$REPOSITORY_NAME".`hostname`.service

# Install needed packages
rpms="make gcc docker libicu wget fping unzip jq"
sudo ZYPP_LOCK_TIMEOUT=300 zypper --gpg-auto-import-keys ref
grep SLES /etc/os-release \
  && rpms+=" git-core"    \
  || rpms+=" git"
sudo ZYPP_LOCK_TIMEOUT=300 zypper --non-interactive in -y $rpms

# Enable docker service
sudo systemctl enable docker
sudo systemctl start docker

# Docker post-install step (needs re-login)
sudo usermod -aG docker $USER

# Install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl
sudo mv kubectl /usr/bin

# Setup github worker
mkdir -p actions-runner && cd actions-runner
curl -o actions-runner-linux-x64-2.288.1.tar.gz -L https://github.com/actions/runner/releases/download/v2.288.1/actions-runner-linux-x64-2.288.1.tar.gz
tar xzf ./actions-runner-linux-x64-2.288.1.tar.gz

# Make non-interactive
sed -i 's/Runner.Listener configure/Runner.Listener configure --unattended/' config.sh
./config.sh --url "$GITHUB_REPOSITORY_URL" --token "$GITHUB_RUNNER_TOKEN" $runner_labels

# Configure and enable service
sudo ./svc.sh install
sudo sed -i '/^\[Service\]/a RestartSec=5s' /etc/systemd/system/"$ACTIONS_RUNNER_SERVICE"
sudo sed -i '/^\[Service\]/a Restart=always' /etc/systemd/system/"$ACTIONS_RUNNER_SERVICE"
sudo systemctl daemon-reload
sudo systemctl enable "$ACTIONS_RUNNER_SERVICE"
sudo systemctl start "$ACTIONS_RUNNER_SERVICE"
