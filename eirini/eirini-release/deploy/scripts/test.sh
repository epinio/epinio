#!/bin/bash

check_app_exists() {
  echo "Checking if app exists..."
  for i in $(seq 60); do
    echo "Attempt #$i"
    sleep 1
    if [ $(kubectl -n eirini-workloads get pods | grep Running | grep the-app-guid | wc -l) -eq 1 ]; then
      echo "+-------------------------------"
      echo "| SUCCESS"
      echo "+-------------------------------"
      return
    fi
  done

  echo "+-------------------------------"
  echo "| FAILED"
  echo "+-------------------------------"
  exit 1
}

cleanup() {
  kubectl -n eirini-workloads delete lrps --all
  kubectl -n eirini-workloads delete statefulsets --all
}

test_api() {
  tls_crt="$(kubectl get secret -n eirini-core eirini-certs -o json | jq -r '.data["tls.crt"]' | base64 -d)"
  tls_key="$(kubectl get secret -n eirini-core eirini-certs -o json | jq -r '.data["tls.key"]' | base64 -d)"
  tls_ca="$(kubectl get secret -n eirini-core eirini-certs -o json | jq -r '.data["tls.ca"]' | base64 -d)"

  eirini_host="$(kubectl -n eirini-core get service eirini-external -ojsonpath="{.status.loadBalancer.ingress[0].ip}")"

  echo "Creating an app via API"
  curl --cacert <(echo "$tls_ca") --key <(echo "$tls_key") --cert <(echo "$tls_crt") -k "https://$eirini_host:8085/apps/testapp" -X PUT -H "Content-Type: application/json" -d '{"guid": "the-app-guid","version": "0.0.0","ports" : [8080],"lifecycle": {"docker_lifecycle": {"image": "busybox","command": ["/bin/sleep", "100"]}},"instances": 1}'

  check_app_exists
}

test_crd() {
  echo "Creating an app via CRD"
  cat <<EOF | kubectl apply -f -
apiVersion: eirini.cloudfoundry.org/v1
kind: LRP
metadata:
  name: testapp
  namespace: eirini-workloads
spec:
  GUID: "the-app-guid"
  version: "version-1"
  instances: 1
  lastUpdated: "never"
  ports:
  - 8080
  image: "eirini/dorini"
EOF

  check_app_exists
}

echo "Cleaning up..."
kubectl -n eirini-workloads delete statefulsets --all
kubectl -n eirini-workloads delete lrps --all

cluster_name=$(kubectl config current-context | cut -d _ -f 4)
echo "Using cluster '$cluster_name'"
trap cleanup EXIT

test_api
test_crd
