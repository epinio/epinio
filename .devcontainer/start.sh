#!/bin/bash
set -e

kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 8443:443 --address 0.0.0.0 &