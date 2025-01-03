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

VERSION="5.7.5"

URL="https://github.com/k3d-io/k3d/releases/download/v${VERSION}/k3d-linux-amd64"
SHA256="5d3f22817d9e163ab6ed43572189dd49fe724d7a6948075b570067747eca8d3f"

pushd "$TMP_DIR" > /dev/null
wget -q "$URL" -O k3d
echo "${SHA256} k3d" | sha256sum -c

chmod +x k3d
mv k3d "${OUTPUT_DIR}/k3d"
popd > /dev/null
