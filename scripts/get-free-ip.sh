#!/bin/bash

set -e

# Get host (main!) IP (for example 192.168.4.100)
ETH_DEV=$(ip route | awk '/default via / { print $5 }')
HOST_IP=$(ip a s ${ETH_DEV}                                                \
	  | egrep -o 'inet [0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}' \
	  | cut -d' ' -f2)

# Get Host network in CIDR format (for example 192.168.4.0/24)
NETWORK=$(ip route | awk "/link src ${HOST_IP} / { print \$1 }")

# Get a list of free IP in NETWORK and take the last one (for example 192.168.4.254)
FREE_IP=$(sudo fping -a -g ${NETWORK} 2>&1               \
          | awk '/^ICMP Host Unreachable/ { print $NF }' \
          | sort -u -t . -k 3,3n -k 4,4n                 \
          | tail -n2)

# '\n' should be removed!
echo -n ${FREE_IP} | tr -s '[[:blank:]]' '-'
