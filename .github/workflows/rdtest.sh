#!/bin/bash

# Check priviledges
PRIVILEGES=$([ -r /dev/kvm ] && [ -w /dev/kvm ] || echo 'insufficient privileges')
if [ ${PRIVILEGES}="insufficient privileges" ]; then 
sudo usermod -a -G kvm "$USER" 
else
echo "User has correct privileges for kvm"
fi

## Install RD
curl -s https://download.opensuse.org/repositories/isv:/Rancher:/stable/deb/Release.key | gpg --dearmor | sudo dd status=none of=/usr/share/keyrings/isv-rancher-stable-archive-keyring.gpg
echo 'deb [signed-by=/usr/share/keyrings/isv-rancher-stable-archive-keyring.gpg] https://download.opensuse.org/repositories/isv:/Rancher:/stable/deb/ ./' | sudo dd status=none of=/etc/apt/sources.list.d/isv-rancher-stable.list
sudo apt update
sudo apt install -y rancher-desktop

## Start Rancher Desktop
rancher-desktop >/dev/null 2>&1  &
sleep 40

## Stop RD to later change to moby
rdctl shutdown
sleep 40

## Change Engine
rdctl set --container-engine docker

## Start again RD
rdctl start
sleep 60

## Install Epinio CLI
curl -o epinio -L https://github.com/epinio/epinio/releases/download/v1.1.0/epinio-linux-x86_64
chmod +x epinio
sudo mv ./epinio /usr/local/bin/epinio

## Install Epino
kubectl create namespace cert-manager 
helm repo add jetstack https://charts.jetstack.io 
helm repo update 
helm install cert-manager --namespace cert-manager jetstack/cert-manager  --set installCRDs=true  --set "extraArgs[0]=--enable-certificate-owner-ref=true"
helm repo add epinio https://epinio.github.io/helm-charts 
MYEPINIODOMAIN=`kubectl get svc -n kube-system traefik | awk '{print $4}' | tail --lines=+2` 
helm upgrade --install epinio -n epinio --create-namespace epinio/epinio  --set global.domain=${MYEPINIODOMAIN}.sslip.io 

sleep 30
## Check it can login
epinio login -u admin https://epinio.${MYEPINIODOMAIN}.sslip.io
read -sp 'Password: ' password

# ## Remove Rancher Desktop
# sudo apt remove --autoremove -y rancher-desktop 
# sudo rm /etc/apt/sources.list.d/isv-rancher-stable.list 
# sudo rm /usr/share/keyrings/isv-rancher-stable-archive-keyring.gpg 
# sudo apt update 
# sudo rm -rf .rd 
# sudo rm -rf .local/share/rancher-desktop/ 