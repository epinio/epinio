#!/usr/bin/env bash
source scripts/include/setup.sh

require_tools minikube

: "${VM_CPUS:=4}"
: "${VM_MEMORY:=16384}"
: "${VM_DISK_SIZE:=120g}"

: "${MINIKUBE_ISO_VERSION:=0.1.8}"
: "${MINIKUBE_ISO_URL:=https://github.com/f0rmiga/opensuse-minikube-image/releases/download/v${MINIKUBE_ISO_VERSION}/minikube-openSUSE.x86_64-${MINIKUBE_ISO_VERSION}.iso}"

if ! minikube status > /dev/null; then
    # shellcheck disable=SC2086
    minikube start \
             --kubernetes-version "${K8S_VERSION}" \
             --cpus "${VM_CPUS}" \
             --memory "${VM_MEMORY}" \
             --disk-size "${VM_DISK_SIZE}" \
             --iso-url "${MINIKUBE_ISO_URL}" \
             ${VM_DRIVER:+--vm-driver "${VM_DRIVER}"} \
             --extra-config=apiserver.runtime-config=settings.k8s.io/v1alpha1=true \
             --extra-config=apiserver.enable-admission-plugins=MutatingAdmissionWebhook,PodPreset \
             ${MINIKUBE_EXTRA_OPTIONS:-}

    # Enable hairpin by setting the docker0 promiscuous mode on.
    minikube ssh -- "sudo ip link set docker0 promisc on"

    minikube addons enable dashboard
    minikube addons enable metrics-server
    minikube addons enable metallb

    # Configure metallb addon
    MINIKUBE_IP=($(minikube ip))
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: metallb-system
  name: config
data:
  config: |
    address-pools:
    - name: default
      protocol: layer2
      addresses:
      - ${MINIKUBE_IP}/28
EOF

else
    echo "Minikube is already started"
fi
