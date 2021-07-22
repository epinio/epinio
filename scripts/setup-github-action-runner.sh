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
if [ $(cat /etc/os-release | grep SLES) ]; then
  echo -e "\x1B[31m In order to install tmux, you need to register with SUSE Package Hub"
  sudo zypper --gpg-auto-import-keys ref && sudo zypper --non-interactive in -y make gcc docker git-core libicu tmux wget
else
  sudo zypper --gpg-auto-import-keys ref && sudo zypper --non-interactive in -y make gcc docker git libicu tmux wget
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
echo "tmux new-session -d -s runner 'cd actions-runner && ./run.sh'"
