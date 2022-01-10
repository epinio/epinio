#!/bin/bash

set -e

for REPO in $(helm repo list 2>/dev/null | awk '(NR>1) { print $1 }')
do
  echo -n "Removing helm repo ${REPO}: "
  helm repo remove ${REPO} 2>/dev/null || echo KO
done

exit 0
