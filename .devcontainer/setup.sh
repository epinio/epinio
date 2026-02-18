#!/bin/bash
set -e

# Install k3d
curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash

# Wait for Docker to be ready
while ! docker info > /dev/null 2>&1; do
  echo "Waiting for Docker..."
  sleep 1
done

# Delete existing cluster for a clean start
# k3d cluster delete epinio 2>/dev/null || true
# k3d cluster create epinio --wait \
#     -p "80:80@loadbalancer" \
#     -p "443:443@loadbalancer" \
#     --k3s-arg "--disable=traefik@server:*"

k3d cluster list | grep -q epinio || k3d cluster create epinio --wait \
    -p "80:80@loadbalancer" \
    -p "443:443@loadbalancer" \
    --k3s-arg "--disable=traefik@server:*"

# Write and export kubeconfig
export KUBECONFIG="$HOME/.kube/config"
mkdir -p "$HOME/.kube"
k3d kubeconfig get epinio > "$KUBECONFIG"

export EPINIO_SYSTEM_DOMAIN=127.0.0.1.sslip.io

# Install cert-manager
echo "Installing Cert Manager..."
helm repo add cert-manager https://charts.jetstack.io
helm repo update
helm upgrade --install cert-manager --create-namespace -n cert-manager \
    --set crds.enabled=true \
    --set crds.keep=false \
    --set extraArgs[0]=--enable-certificate-owner-ref=true \
    cert-manager/cert-manager --version 1.18.1 \
    --wait

# Dynamic storage provisioner
kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/master/deploy/local-path-storage.yaml
kubectl patch storageclass local-path -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'

# Install nginx ingress controller
echo "Installing nginx ingress..."
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
    --namespace ingress-nginx \
    --create-namespace \
    --set controller.ingressClassResource.default=true \
    --set controller.admissionWebhooks.enabled=false \
    --wait

# Install Epinio
echo "Installing Epinio..."
helm repo add epinio https://epinio.github.io/helm-charts
helm repo update
helm upgrade --install epinio epinio/epinio --namespace epinio --create-namespace \
    --set global.domain="${EPINIO_SYSTEM_DOMAIN}" \
    --set server.disableTracking="true" \
    --set ingress.nginxSSLRedirect="false" \
    --set "extraEnv[0].name=KUBE_API_QPS" --set-string "extraEnv[0].value=50" \
    --set "extraEnv[1].name=KUBE_API_BURST" --set-string "extraEnv[1].value=100" \
    --wait

echo "============================================"
echo "Setup complete!"
echo ""
echo "To access Epinio from your host browser, ensure these entries are in your host machine's /etc/hosts:"
echo ""
echo "  127.0.0.1  epinio.127.0.0.1.sslip.io auth.127.0.0.1.sslip.io"
echo ""
echo "Then visit: https://epinio.127.0.0.1.sslip.io:8443"
echo "============================================"
kubectl get all -n epinio