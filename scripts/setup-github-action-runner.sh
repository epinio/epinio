#!/bin/bash

# This script can be used to create a Github Action Runner on an openSUSE or SLE
# distro. It installs all the needed dependencies to run the acceptance tests
# and sets up docker and the runner as a service itself.

set -e

if [ -z "$GITHUB_RUNNER_TOKEN" ]; then
  echo "The variable GITHUB_RUNNER_TOKEN is empty. Exiting"
  exit 1
fi

# Install needed packages
sudo zypper --gpg-auto-import-keys ref
if [ $(cat /etc/os-release | grep SLES) ]; then
   rpms="make gcc docker git-core libicu wget"
   sudo zypper --non-interactive in -y $rpms
  else
   rpms="make gcc docker git libicu wget"
   sudo zypper --non-interactive in -y $rpms
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
mkdir -p actions-runner && cd actions-runner
curl -o actions-runner-linux-x64-2.278.0.tar.gz -L https://github.com/actions/runner/releases/download/v2.278.0/actions-runner-linux-x64-2.278.0.tar.gz
tar xzf ./actions-runner-linux-x64-2.278.0.tar.gz

# Make non-interactive
sed -i 's/Runner.Listener configure/Runner.Listener configure --unattended/' config.sh
./config.sh --url https://github.com/epinio/epinio --token $GITHUB_RUNNER_TOKEN

# Configure and enable Service
sudo ./svc.sh install
sudo sed -i '/^\[Service\]/a RestartSec=5s' /etc/systemd/system/actions.runner.epinio-epinio.`hostname`.service
sudo sed -i '/^\[Service\]/a Restart=always' /etc/systemd/system/actions.runner.epinio-epinio.`hostname`.service
sudo systemctl daemon-reload
sudo systemctl enable actions.runner.epinio-epinio.`hostname`.service
sudo systemctl start actions.runner.epinio-epinio.`hostname`.service
