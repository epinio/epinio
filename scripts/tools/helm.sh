#!/bin/bash
set -e
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

VERSION="3.9.0"

URL="https://get.helm.sh/helm-v${VERSION}-linux-amd64.tar.gz"
SHA256="1484ffb0c7a608d8069470f48b88d729e88c41a1b6602f145231e8ea7b43b50a"

pushd "$TMP_DIR" > /dev/null
wget -q "$URL" -O "helm.tar.gz"
echo "${SHA256} helm.tar.gz" | sha256sum -c

mkdir -p helm
tar xvf "helm.tar.gz" -C helm
mv helm/*/helm "${OUTPUT_DIR}/helm"
popd > /dev/null
