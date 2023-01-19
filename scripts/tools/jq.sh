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

VERSION="1.6"

URL="https://github.com/stedolan/jq/releases/download/jq-${VERSION}/jq-linux64"
SHA256="af986793a515d500ab2d35f8d2aecd656e764504b789b66d7e1a0b727a124c44"

pushd "$TMP_DIR" > /dev/null
wget -q "$URL" -O jq
echo "${SHA256} jq" | sha256sum -c

chmod +x jq
mv jq "${OUTPUT_DIR}/jq"
popd > /dev/null
