#!/bin/sh

# Inspired by this script:
# https://github.com/k3d-io/k3d-demo/blob/main/.drone.yml

set -e

ensure_epinio_cluster() {
  if ! k3d cluster get epinio; then
    k3d cluster create epinio -p '80:80@server:0' -p '443:443@server:0'
  fi
}

ensure_cert_manager() {
  if ! helm status -n cert-manager cert-manager; then
    helm upgrade --install -n cert-manager --create-namespace cert-manager jetstack/cert-manager \
            --set installCRDs=true \
            --set extraArgs[0]=--enable-certificate-owner-ref=true
  fi
}

ensure_epinio() {
  if ! helm status -n epinio epinio; then
    helm upgrade --install epinio -n epinio --create-namespace epinio/epinio \
      --set global.domain=127.0.0.1.sslip.io
  fi
}

# Run the k3d entrypoint (start docker in the background)
nohup dockerd-entrypoint.sh > /dev/null 2>&1 &

printf "Waiting for container runtime to be ready."
until docker ps > /dev/null 2>&1; do
  printf "."
  sleep 1s;
done
echo "Done"

echo "Creating a cluster for epinio"
ensure_epinio_cluster

echo "Checking with kubectl"
kubectl get nodes

echo "Adding helm repositories"
helm repo add jetstack https://charts.jetstack.io
helm repo add epinio https://epinio.github.io/helm-charts
helm repo update

echo "Installing cert-manager"
ensure_cert_manager

echo "Installing Epinio"
ensure_epinio

trap : TERM INT; sleep infinity & wait
