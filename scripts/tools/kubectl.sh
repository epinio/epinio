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

VERSION="1.29.15"

URL="https://dl.k8s.io/release/v${VERSION}/bin/linux/amd64/kubectl"
SHA256="3473e14c7b024a6e5403c6401b273b3faff8e5b1fed022d633815eb3168e4516"

pushd "$TMP_DIR" > /dev/null
wget -q "$URL" -O kubectl
echo "${SHA256} kubectl" | sha256sum -c

chmod +x kubectl
mv kubectl "${OUTPUT_DIR}/kubectl"
popd > /dev/null
