#!/bin/bash

# CARRIER_ACCEPTANCE_KUBECONFIG_DIR should be a directory with kubeconfig files
# pointing to kubernetes clusters. It acts as a cluster pool and when set, it
# it defines how many ginkgo nodes will be used. Each ginkgo node will use a
# different cluster from the pool.
# If it's not set, we run on a hardcoded number of Ginkgo nodes and a cluster
# is spawn for each one of the nodes. This means the clusters will be "cold",
# or in other words tests will be slower because all the images have to be
# downloaded.
if [ -z ${CARRIER_ACCEPTANCE_KUBECONFIG_DIR+x} ]; then
  #ginkgo -p -stream acceptance/.
  ginkgo -nodes 2 -stream acceptance/.
else
  # Count the files in the CARRIER_ACCEPTANCE_KUBECONFIG_DIR directory
  GINKGO_NODES=$(ls -p "${CARRIER_ACCEPTANCE_KUBECONFIG_DIR}" | grep -v / | wc -l)
  ginkgo -nodes ${GINKGO_NODES} -stream acceptance/.
fi
