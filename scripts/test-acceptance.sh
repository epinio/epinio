#!/bin/bash

# Don't set CARRIER_ACCEPTANCE_KUBECONFIG when using multiple ginkgo
# nodes because they will all use the same cluster. This will lead to flaky
# tests.
if [ -z ${CARRIER_ACCEPTANCE_KUBECONFIG+x} ]; then
  #ginkgo -p -stream acceptance/.
  ginkgo -nodes 3 -stream acceptance/.
else
  ginkgo acceptance/.
fi
