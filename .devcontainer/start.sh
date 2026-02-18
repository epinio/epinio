#!/bin/bash

touch /tmp/dev-reload-output.log
tail -f /tmp/dev-reload-output.log &

export KUBECONFIG="$HOME/.kube/config"

echo "Waiting for ingress-nginx to be ready..."
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=controller -n ingress-nginx --timeout=120s

echo "Starting port-forward..."
nohup kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 8443:443 --address 0.0.0.0 > /tmp/port-forward.log 2>&1 &
disown

# Wait for port-forward to be ready
echo "Waiting for port-forward to be ready..."
for i in $(seq 1 30); do
  if curl -sk https://localhost:8443 > /dev/null 2>&1; then
    echo "Port-forward is ready!"
    break
  fi
  sleep 1
done