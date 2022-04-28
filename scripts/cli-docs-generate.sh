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

# Fix up the app chart docs, hide the operator commands
#rm "${destination}"/epinio_app_chart_create.md
#rm "${destination}"/epinio_app_chart_delete.md
#cat "${destination}"/epinio_app_chart.md | grep -v _create | grep -v _delete > $$
#mv $$ "${destination}"/epinio_app_chart.md

echo /Done
exit
