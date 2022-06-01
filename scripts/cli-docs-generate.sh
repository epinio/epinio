#!/bin/bash

set -e

destination="$1"

echo Generating into ${destination} ...

rm -f "${destination}"/*

go run internal/cli/docs/generate-cli-docs.go "${destination}"/

# Fix ${HOME} references to proper `~`.
perl -pi -e "s@${HOME}@~@" "${destination}"/*md

echo /Done
exit
