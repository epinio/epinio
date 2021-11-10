#!/bin/bash

set -e

destination="$1"

echo Generating into ${destination} ...

rm -f "${destination}"/*

go run internal/cli/docs/generate-cli-docs.go "${destination}"/

# Fix ${HOME} references to proper `~`.
perl -pi -e "s@${HOME}@~@" "${destination}"/*md

# Fix the cross-links to match hierarchy and mdbook expectations.
sed -i 's@(\.\./\(.*\))@(\1.md)@' "${destination}"/*md

# Drop the HUGO specific annotations from file heads
for md in "${destination}"/*md
do
    tail --lines +6 $md > $$
    mv $$ $md
done

echo /Done
exit
