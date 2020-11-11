#!/usr/bin/env bash
source scripts/include/setup.sh

# Make sure we have an exact version match for *all* defined tools.
PINNED_TOOLS=true require_tools "${TOOLS[@]}"
