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

version="test-$(git describe --tags)"
imageEpServer="ghcr.io/epinio/epinio-server"
imageUnpacker="ghcr.io/epinio/epinio-unpacker"

# Build images
docker build -t "${imageEpServer}:${version}" -t "${imageEpServer}:latest" -f images/Dockerfile .
docker build -t "${imageUnpacker}:${version}" -t "${imageUnpacker}:latest" -f images/unpacker-Dockerfile .
