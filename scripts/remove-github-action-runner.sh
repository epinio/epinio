#!/bin/bash

# This script can be used to remove a Github Action Runner on an openSUSE or SLE
# distro. It will unconfigure the service and unregister the runner.

set -e

if [ -z "$GITHUB_RUNNER_TOKEN" ]; then
  echo "The variable GITHUB_RUNNER_TOKEN is empty. Exiting"
  exit 1
fi

cd actions-runner
sudo systemctl stop actions.runner.epinio-epinio.`hostname`.service
sudo ./svc.sh uninstall
./config.sh remove --token $GITHUB_RUNNER_TOKEN
