#!/bin/bash

ORG=epinio
PROJECT=epinio
ARTI=epinio-linux-x86_64

echo
echo Locating latest ...
echo = Release
LATEST_RELEASE="$(curl -L -s -H 'Accept: application/json' https://github.com/${ORG}/${PROJECT}/releases/latest)"
echo = $LATEST_RELEASE
echo = Version
LATEST_VERSION="$(echo "${LATEST_RELEASE}" | jq .tag_name | sed -e 's/"//g')"
echo = $LATEST_VERSION
echo = Artifact
ARTIFACT_URL="https://github.com/${ORG}/${PROJECT}/releases/download/${LATEST_VERSION}/${ARTI}"
echo = $ARTIFACT_URL

echo
echo Retrieving artifact ...
curl -L -o epinio.bin $ARTIFACT_URL
chmod u+x epinio.bin

echo
echo Version Old: $(dist/epinio-linux-amd64 version)
echo Version Got: $(./epinio.bin version)

cp epinio.bin dist/epinio-linux-amd64

echo Version Now: $(dist/epinio-linux-amd64 version)

echo
dist/epinio-linux-amd64 info
