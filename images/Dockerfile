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

FROM alpine AS certs
RUN apk --update --no-cache add ca-certificates

FROM scratch
COPY --from=certs /etc/ssl/certs /etc/ssl/certs

# default, if running outside of gorelease with a self-compiled binary
ARG DIST_BINARY=dist/epinio-linux-amd64
ARG SWAGGER_FILE=docs/references/api/swagger.json

COPY ${DIST_BINARY} /epinio
COPY ${SWAGGER_FILE} /swagger.json

ENTRYPOINT ["/epinio"]