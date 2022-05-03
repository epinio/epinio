#!/bin/bash

set -e

destination="$1"

echo Generating into ${destination} ...

rm -f "${destination}"/*

go run internal/cli/docs/generate-cli-docs.go "${destination}"/

# Fix ${HOME} references to proper `~`.
perl -pi -e "s@${HOME}@~@" "${destination}"/*md

# Fix the cross-links to match hierarchy and tooling expectations.
sed -i 's@(\.\./\(.*\))@(./\1.md)@' "${destination}"/*md

# Rework the YAML header/annotations. Fix `push` link.
for md in "${destination}"/*md
do
    echo -n .
    ( cat $md | grep -v linkTitle: | grep -v weight:
    ) | sed -e 's|epinio_app_push.md|epinio_push.md|' \
      > $$
    mv $$ $md
done

echo /Done
exit
