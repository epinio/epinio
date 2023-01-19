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

set -e

destination="$1"

echo Generating into ${destination} ...

rm -rf "${destination}"/*

go run internal/cli/docs/generate-cli-docs.go "${destination}"/

# Fix ${HOME} references to proper `~`.
find ${destination} -type f -name \*.md -exec perl -pi -e "s@${HOME}@~@" {} +

echo /Done
exit
