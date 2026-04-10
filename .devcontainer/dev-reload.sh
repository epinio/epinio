#!/bin/bash
set -e

export KUBECONFIG="$HOME/.kube/config"

# Skip if cluster isn't ready yet
if ! kubectl get deployment epinio-server -n epinio > /dev/null 2>&1; then
  echo "⏳ Cluster not ready yet, skipping reload..."
  exit 0
fi

LOCKFILE="/tmp/dev-reload.lock"

if [ -f "$LOCKFILE" ]; then
  echo "⏳ Reload already in progress, skipping..."
  exit 0
fi

trap "rm -f $LOCKFILE" EXIT
touch "$LOCKFILE"

echo ""
echo "🔄 ============================================"
echo "🔄  Dev Reload Started at $(date +%H:%M:%S)"
echo "🔄 ============================================"

echo "🔨 Building Go binary..."
make build

echo "🚀 Patching deployment..."
bash scripts/patch-epinio-deployment.sh

echo ""
echo "✅ ============================================"
echo "✅  Reload Complete at $(date +%H:%M:%S)"
echo "✅ ============================================"
echo ""

echo -e "\a"  # terminal bell - VS Code shows a notification
