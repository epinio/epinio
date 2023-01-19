#!/bin/bash
# Copyright © 2021 - 2023 SUSE LLC
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

ORG=epinio
PROJECT=epinio
ARTI=epinio-windows-x86_64.zip

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
curl -L -o epinio.zip $ARTIFACT_URL
unzip epinio.zip -x LICENSE README.md
chmod u+x  epinio.exe

echo
if test -f dist/epinio-windows-amd64.exe ; then
    echo Version Old: $(dist/epinio-windows-amd64.exe version)
else
    echo Version Old: n/a
fi
echo Version Got: $(./epinio.exe version)

cp epinio.exe dist/epinio-windows-amd64.exe

echo Version Now: $(dist/epinio-windows-amd64.exe version)

# query cluster. may not exist
echo
dist/epinio-windows-amd64.exe info || true
