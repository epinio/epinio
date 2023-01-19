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

VERSION="5.0.0"

URL="https://github.com/rancher/k3d/releases/download/v${VERSION}/k3d-linux-amd64"
SHA256="6744bfd5cea611dc3f2a24da7b25a28fd0dd4b78c486193c238d55619d7b7c43"

pushd "$TMP_DIR" > /dev/null
wget -q "$URL" -O k3d
echo "${SHA256} k3d" | sha256sum -c

chmod +x k3d
mv k3d "${OUTPUT_DIR}/k3d"
popd > /dev/null
