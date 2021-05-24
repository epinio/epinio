#!/usr/bin/env bash
source scripts/include/setup.sh

# If arguments are passed to this script, those should be the tools to install.
# If no arguments are passed, all tools are installed.
if [ "$#" -gt 0 ]; then
  TOOLS=("$@")
fi

# Make sure we have an exact version match for *all* defined tools.
PINNED_TOOLS=true require_tools "${TOOLS[@]}"
