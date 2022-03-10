#!/bin/bash

# This script can be used to remove a Github Action Runner on an openSUSE or SLE
# distro. It will unconfigure the configuration and unregister the runner.
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
ACTIONS_RUNNER_SERVICE=actions.runner."$REPOSITORY_NAME".`hostname`.configuration

cd actions-runner
sudo systemctl stop "$ACTIONS_RUNNER_SERVICE"
sudo ./svc.sh uninstall
./config.sh remove --token "$GITHUB_RUNNER_TOKEN"
