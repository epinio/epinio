#!/usr/bin/env bash
set -e

# Set up epinio alias
grep -q 'alias epinio=' ~/.bash_aliases 2>/dev/null || echo 'alias epinio="/workspaces/epinio/dist/epinio-linux-amd64"' >> ~/.bash_aliases

# Install k3d
curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash

# Wait for Docker to be ready
while ! docker info > /dev/null 2>&1; do
  echo "Waiting for Docker..."
  sleep 1
done

# Cluster setup
MARKER="/home/vscode/.cluster-initialized"

if [ -f "$MARKER" ]; then
  echo "Existing cluster detected, preserving resources..."
else
  echo "Fresh setup, creating new cluster..."
  k3d cluster delete epinio 2>/dev/null || true
  k3d cluster create epinio --wait \
      -p "80:80@loadbalancer" \
      -p "443:443@loadbalancer" \
      --k3s-arg "--disable=traefik@server:*" \
      -v /var/run/docker.sock:/var/run/docker.sock
  touch "$MARKER"
fi

# Write and export kubeconfig
export KUBECONFIG="$HOME/.kube/config"
mkdir -p "$HOME/.kube"
k3d kubeconfig get epinio > "$KUBECONFIG"

# Connect this devcontainer to the k3d network so kubectl can reach the API server directly
# by container IP. With docker-outside-of-docker, k3d runs as a sibling on the host daemon,
# so 0.0.0.0 in the kubeconfig isn't routable from inside the devcontainer.
CONTAINER_ID=$(hostname)
docker network connect k3d-epinio "$CONTAINER_ID" 2>/dev/null || true
K3D_SERVER_IP=$(docker inspect k3d-epinio-server-0 \
  --format '{{(index .NetworkSettings.Networks "k3d-epinio").IPAddress}}')
sed -i "s|server: https://0\.0\.0\.0:[0-9]*|server: https://${K3D_SERVER_IP}:6443|g" "$KUBECONFIG"

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

# Fix field ownership conflicts from patch script
if kubectl get deployment epinio-server -n epinio > /dev/null 2>&1; then
  echo "Resetting field ownership for epinio-server..."
  kubectl apply --server-side --force-conflicts --field-manager=helm \
    -f <(kubectl get deployment epinio-server -n epinio -o yaml) 2>/dev/null || true
fi

# Install Epinio
echo "Installing Epinio..."
helm repo add epinio https://epinio.github.io/helm-charts
helm repo update

HELM_COMMON_ARGS=(
    --namespace epinio --create-namespace
    --set global.domain="${EPINIO_SYSTEM_DOMAIN}"
    --set server.disableTracking="true"
    --set ingress.nginxSSLRedirect="false"
    --set server.stagingWorkloads.dockerSocketPath="/var/run/docker.sock"
    --set "extraEnv[0].name=KUBE_API_QPS" --set-string "extraEnv[0].value=50"
    --set "extraEnv[1].name=KUBE_API_BURST" --set-string "extraEnv[1].value=100"
    --wait
)

    # --set server.stagingWorkloads.dockerSocketPath="/var/run/docker.sock" \

if [ "${EPINIO_CHART_SOURCE}" = "local" ]; then
    echo "Using local helm chart..."
    helm upgrade --install epinio helm-charts/chart/epinio "${HELM_COMMON_ARGS[@]}"
else
    echo "Using remote helm chart..."
    if [ -n "${EPINIO_CHART_VERSION}" ]; then
        echo "Version: ${EPINIO_CHART_VERSION}"
        helm upgrade --install epinio epinio/epinio --version "${EPINIO_CHART_VERSION}" "${HELM_COMMON_ARGS[@]}"
    else
        echo "Version: latest"
        helm upgrade --install epinio epinio/epinio "${HELM_COMMON_ARGS[@]}"
    fi
fi

# helm upgrade --install epinio epinio/epinio --namespace epinio --create-namespace \
#     --set global.domain="${EPINIO_SYSTEM_DOMAIN}" \
#     --set server.disableTracking="true" \
#     --set ingress.nginxSSLRedirect="false" \
#     --set "extraEnv[0].name=KUBE_API_QPS" --set-string "extraEnv[0].value=50" \
#     --set "extraEnv[1].name=KUBE_API_BURST" --set-string "extraEnv[1].value=100" \
#     --wait

kubectl get all -n epinio

# Apply local changes
bash .devcontainer/dev-reload.sh

echo "============================================"
echo "Setup complete with local changes applied!"
echo ""
echo "To access Epinio from your host browser, ensure these entries are in your host machine's /etc/hosts:"
echo ""
echo "  127.0.0.1  epinio.127.0.0.1.sslip.io auth.127.0.0.1.sslip.io"
echo ""
echo "Then visit: https://epinio.127.0.0.1.sslip.io:8443"
echo "============================================"