#!/bin/bash

set -e

# UNAME should be darwin or linux
UNAME="$(uname | tr "[:upper:]" "[:lower:]")"

# EPINIO_BINARY is used to execute the installation commands
EPINIO_BINARY="./dist/epinio-"${UNAME}"-amd64"

function check_dependency {
	for dep in "$@"
	do
		if ! [ -x "$(command -v $dep)" ]; then
			echo "Error: ${dep} is not installed." >&2
  			exit 1
		fi
	done

}

function create_docker_pull_secret {
	if [[ "$REGISTRY_USERNAME" != "" && "$REGISTRY_PASSWORD" != ""  ]]; 
	then
		kubectl create secret docker-registry regcred \
			--docker-server https://index.docker.io/v1/ \
			--docker-username $REGISTRY_USERNAME \
			--docker-password $REGISTRY_PASSWORD
	fi
}

# Check Dependencies
check_dependency kubectl helm
# Create docker registry image pull secret
create_docker_pull_secret

# Install Epinio
EPINIO_DONT_WAIT_FOR_DEPLOYMENT=1 "${EPINIO_BINARY}" install --skip-default-namespace

# Patch Epinio
./scripts/patch-epinio-deployment.sh
sleep 10

# Create Org
"${EPINIO_BINARY}" namespace create workspace
"${EPINIO_BINARY}" target workspace

# Create in-cluster services
"${EPINIO_BINARY}" enable services-incluster

# Create google services
cat <<'EOF' > /tmp/service_account.json
{
	"type": "service_account",
	"project_id": "myproject",
	"private_key_id": "somekeyid",
	"private_key": "someprivatekey",
	"client_email": "client@example.com",
	"client_id": "clientid",
	"auth_uri": "https://accounts.google.com/o/oauth2/auth",
	"token_uri": "https://oauth2.googleapis.com/token",
	"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
	"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/client%40example.com"
}
EOF
"${EPINIO_BINARY}" enable services-google --service-account-json /tmp/service_account.json


# Check Epinio Insllation
"${EPINIO_BINARY}" info
