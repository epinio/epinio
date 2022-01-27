#!/bin/bash

set -e

# Get host (main!) IP (for example 192.168.4.100)
ETH_DEV=$(ip route | awk '/default via / { print $5 }')
HOST_IP=$(ip a s ${ETH_DEV}                                                \
          | egrep -o 'inet [0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}' \
          | cut -d' ' -f2)

# '\n' should be removed!
echo -n ${HOST_IP}
