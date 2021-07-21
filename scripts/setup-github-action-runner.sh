#!/bin/bash

# This script can be used to create a Github Action Runner on an openSUSE or SLE
# distro. It installs all the needed dependencies to run the acceptance tests
# and sets up docker and the runner itself.

set -e

if [ -z "$GITHUB_RUNNER_TOKEN" ]; then
  echo "The variable GITHUB_RUNNER_TOKEN is empty. Exiting"
  exit 1
fi

# Install needed packages
if $(cat /etc/os-release | grep SLES); then
  sudo zypper --gpg-auto-import-keys ref && sudo zypper --non-interactive in -y make gcc docker git-core libicu screen
else $(cat /etc/os-release | grep openSUSE); then
  sudo zypper --gpg-auto-import-keys ref && sudo zypper --non-interactive in -y make gcc docker git libicu tmux
fi

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
mkdir actions-runner && cd actions-runner
curl -o actions-runner-linux-x64-2.278.0.tar.gz -L https://github.com/actions/runner/releases/download/v2.278.0/actions-runner-linux-x64-2.278.0.tar.gz
tar xzf ./actions-runner-linux-x64-2.278.0.tar.gz

# Make non-interactive
sed -i 's/Runner.Listener configure/Runner.Listener configure --unattended/' config.sh
./config.sh --url https://github.com/epinio/epinio --token $GITHUB_RUNNER_TOKEN

echo "Your worker will be ready to be used after you re-login (to be able to call 'docker' as non root)"
echo "After login run:"
if $(cat /etc/os-release | grep SLES); then
  echo "screen -d -m bash -c 'cd actions-runner && ./run.sh'"
else $(cat /etc/os-release | grep openSUSE); then
  echo "tmux new-session -d -s runner 'cd actions-runner && ./run.sh'"
fi
