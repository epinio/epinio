#!/bin/bash

set -eu

echo "Will now generate tls.ca tls.crt and tls.key files"

mkdir -p keys
trap 'rm -rf keys' EXIT

otherDNS=$1

pushd keys
{
  openssl req -x509 -newkey rsa:4096 -keyout tls.key -out tls.crt -nodes -subj '/CN=localhost' -addext "subjectAltName = DNS:$otherDNS" -days 365

  if ! kubectl -n eirini-core get secret eirini-certs >/dev/null 2>&1; then
    echo "Creating the secret in your kubernetes cluster"
    kubectl create secret -n eirini-core generic eirini-certs --from-file=tls.crt=./tls.crt --from-file=ca.crt=./tls.crt --from-file=tls.key=./tls.key
  fi

  if ! kubectl -n eirini-core get secret loggregator-certs >/dev/null 2>&1; then
    kubectl create secret -n eirini-core generic loggregator-certs --from-file=tls.crt=./tls.crt --from-file=ca.crt=./tls.crt --from-file=tls.key=./tls.key
  fi

  echo "Done!"
}
popd
