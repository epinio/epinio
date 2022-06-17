#!/bin/bash

set -e

destination="$1"

echo Generating into ${destination} ...

rm -rf "${destination}"/*

go run internal/cli/docs/generate-cli-docs.go "${destination}"/

# Fix ${HOME} references to proper `~`.
find ${destination} -type f -name \*.md -exec perl -pi -e "s@${HOME}@~@" {} +

echo /Done
exit
