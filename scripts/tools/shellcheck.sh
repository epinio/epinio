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

VERSION="0.7.0"

URL="https://github.com/koalaman/shellcheck/releases/download/v${VERSION}/shellcheck-v${VERSION}.linux.x86_64.tar.xz"
SHA256="39c501aaca6aae3f3c7fc125b3c3af779ddbe4e67e4ebdc44c2ae5cba76c847f"

pushd "$TMP_DIR" > /dev/null
wget -q "$URL" -O "shellcheck.tar.gz"
echo "${SHA256} shellcheck.tar.gz" | sha256sum -c

mkdir -p shellcheck
tar xvf "shellcheck.tar.gz" -C shellcheck
mv shellcheck/*/shellcheck "${OUTPUT_DIR}/shellcheck"
popd > /dev/null
