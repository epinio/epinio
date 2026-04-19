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

VERSION="3.19.0"

URL="https://get.helm.sh/helm-v${VERSION}-linux-amd64.tar.gz"
SHA256="a7f81ce08007091b86d8bd696eb4d86b8d0f2e1b9f6c714be62f82f96a594496"

pushd "$TMP_DIR" > /dev/null
wget -q "$URL" -O "helm.tar.gz"
echo "${SHA256} helm.tar.gz" | sha256sum -c

mkdir -p helm
tar xvf "helm.tar.gz" -C helm
mv helm/*/helm "${OUTPUT_DIR}/helm"
popd > /dev/null
