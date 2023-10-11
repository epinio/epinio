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

########################################################################
## Development

# Custom version suffix for use during development. The default, the
# empty string, ensures that CI, releases, etc. are not affected by
# this additional variable. During development on the other hand it
# can be used to provide a string allowing developers to distinguish
# different binaries built from the same base commit. A simple means
# for that would be a timestamp. For example, via
#
# VSUFFIX="-$(date +%Y-%m-%dT%H-%M-%S)" make ...
#
# yielding, for example
#
# % ep version
# Epinio Version: "v0.1.6-16-ge5ad0849-2021-11-18T10-00-27"

VSUFFIX ?= 
VERSION ?= $(shell git describe --tags)$(VSUFFIX)
CGO_ENABLED ?= 0
export LDFLAGS += -X github.com/epinio/epinio/internal/version.Version=$(VERSION)

build: build-amd64

# amd64 variant
build-cover:
	GOARCH="amd64" GOOS="linux" CGO_ENABLED=0 go build -cover -covermode=count -coverpkg ./... $(BUILD_ARGS) -ldflags '$(LDFLAGS)' -o dist/epinio-linux-amd64

build-win: build-windows

build-all: build-amd64 build-arm64 build-arm32 build-windows build-darwin build-darwin-m1

build-all-small:
	@$(MAKE) LDFLAGS+="-s -w" build-all

build-linux-arm: build-arm32
build-arm32:
	GOARCH="arm" GOOS="linux" CGO_ENABLED=$(CGO_ENABLED) go build $(BUILD_ARGS) -ldflags '$(LDFLAGS)' -o dist/epinio-linux-arm32

build-linux-arm64: build-arm64
build-arm64:
	GOARCH="arm64" GOOS="linux" CGO_ENABLED=$(CGO_ENABLED) go build $(BUILD_ARGS) -ldflags '$(LDFLAGS)' -o dist/epinio-linux-arm64

build-linux-amd64: build-amd64
build-amd64:
	GOARCH="amd64" GOOS="linux" CGO_ENABLED=$(CGO_ENABLED) go build $(BUILD_ARGS) -ldflags '$(LDFLAGS)' -o dist/epinio-linux-amd64

build-windows-amd64: build-windows
build-windows:
	GOARCH="amd64" GOOS="windows" CGO_ENABLED=$(CGO_ENABLED) go build $(BUILD_ARGS) -ldflags '$(LDFLAGS)' -o dist/epinio-windows-amd64.exe

build-darwin-amd64: build-darwin
build-darwin:
	GOARCH="amd64" GOOS="darwin" CGO_ENABLED=$(CGO_ENABLED) go build $(BUILD_ARGS) -ldflags '$(LDFLAGS)' -o dist/epinio-darwin-amd64

build-darwin-arm64: build-darwin-m1
build-darwin-m1:
	GOARCH="arm64" GOOS="darwin" CGO_ENABLED=$(CGO_ENABLED) go build $(BUILD_ARGS) -ldflags '$(LDFLAGS)' -o dist/epinio-darwin-arm64

build-linux-s390x: build-s390x
build-s390x:
	GOARCH="s390x" GOOS="linux" CGO_ENABLED=$(CGO_ENABLED) go build $(BUILD_ARGS) -ldflags '$(LDFLAGS)' -o dist/epinio-linux-s390x

build-images: build-linux-amd64
	@./scripts/build-images.sh

compress:
	upx --brute -1 ./dist/epinio-linux-arm32
	upx --brute -1 ./dist/epinio-linux-arm64
	upx --brute -1 ./dist/epinio-linux-amd64
	upx --brute -1 ./dist/epinio-windows-amd64.exe
	upx --brute -1 ./dist/epinio-darwin-amd64
	upx --brute -1 ./dist/epinio-darwin-arm64

test:
	ginkgo --nodes ${GINKGO_NODES} -r -p --cover -race --fail-on-pending --skip-file=acceptance

tag:
	@git describe --tags --abbrev=0

########################################################################
# Acceptance tests

FLAKE_ATTEMPTS ?= 2
GINKGO_NODES ?= 2
GINKGO_POLL_PROGRESS_AFTER ?= 200s
REGEX ?= ""
STANDARD_TEST_OPTIONS= -v --nodes ${GINKGO_NODES} --poll-progress-after ${GINKGO_POLL_PROGRESS_AFTER} --randomize-all --flake-attempts=${FLAKE_ATTEMPTS} --fail-on-pending

acceptance-cluster-delete:
	k3d cluster delete epinio-acceptance
	@if test -f /usr/local/bin/rke2-uninstall.sh; then sudo sh /usr/local/bin/rke2-uninstall.sh; fi

acceptance-cluster-delete-kind:
	kind delete cluster --name epinio-acceptance

acceptance-cluster-setup:
	@./scripts/acceptance-cluster-setup.sh

acceptance-cluster-setup-kind:
	@./scripts/acceptance-cluster-setup-kind.sh

acceptance-cluster-setup-several-k8s-versions:
	@./scripts/acceptance-cluster-setup-several-k8s-versions.sh

test-acceptance: showfocus
	ginkgo ${STANDARD_TEST_OPTIONS} acceptance/. acceptance/api/v1/. acceptance/apps/.

test-acceptance-api: showfocus
	ginkgo ${STANDARD_TEST_OPTIONS} acceptance/api/v1/.

test-acceptance-api-apps: showfocus
	ginkgo ${STANDARD_TEST_OPTIONS} --label-filter "application" acceptance/api/v1/.

test-acceptance-api-services: showfocus
	ginkgo ${STANDARD_TEST_OPTIONS} --label-filter "service" acceptance/api/v1/.

test-acceptance-api-other: showfocus
	ginkgo ${STANDARD_TEST_OPTIONS} --label-filter "!application && !service" acceptance/api/v1/.

test-acceptance-apps: showfocus
	ginkgo ${STANDARD_TEST_OPTIONS} acceptance/apps/.

test-acceptance-cli: showfocus
	ginkgo ${STANDARD_TEST_OPTIONS} acceptance/.

test-acceptance-cli-apps: showfocus
	ginkgo ${STANDARD_TEST_OPTIONS} --label-filter "application" acceptance/.

test-acceptance-cli-services: showfocus
	ginkgo ${STANDARD_TEST_OPTIONS} --label-filter "service" acceptance/.

test-acceptance-cli-other: showfocus
	ginkgo ${STANDARD_TEST_OPTIONS} --label-filter "!application && !service" acceptance/.

test-acceptance-upgrade: showfocus
	ginkgo ${STANDARD_TEST_OPTIONS} --focus "${REGEX}" acceptance/upgrade/.

test-acceptance-install: showfocus
	# TODO support for labels is coming in ginkgo v2
	ginkgo -v --nodes ${GINKGO_NODES} --focus "${REGEX}" --randomize-all --flake-attempts=${FLAKE_ATTEMPTS} acceptance/install/.

test-acceptance-api-apps-critical-endpoints: showfocus
	ginkgo ${STANDARD_TEST_OPTIONS} --focus-file "application_exec_test.go" --label-filter "application" acceptance/api/v1/.
	ginkgo ${STANDARD_TEST_OPTIONS} --focus-file "application_portforward_test.go" --label-filter "application" acceptance/api/v1/.
	ginkgo ${STANDARD_TEST_OPTIONS} --focus-file "application_logs_test.go" --label-filter "application" acceptance/api/v1/.
	ginkgo ${STANDARD_TEST_OPTIONS} --focus-file "service_portforward_test.go" --label-filter "service" acceptance/api/v1/.

showfocus:
	@if test `cat acceptance/*.go acceptance/apps/*.go acceptance/api/v1/*.go | grep -c 'FIt\|FWhen\|FDescribe\|FContext'` -gt 0 ; then echo ; echo 'Focus:' ; grep 'FIt\|FWhen\|FDescribe\|FContext' acceptance/*.go acceptance/apps/*.go acceptance/api/v1/*.go ; echo ; fi

generate:
	go generate ./...

# Assumes that the `docs` checkout is a sibling of the `epinio` checkout
generate-cli-docs:
	@./scripts/cli-docs-generate.sh ../docs/docs/references/commands/cli

lint:
	golangci-lint run --skip-files docs.go

tidy:
	go mod tidy

fmt:
	go fmt ./... ; git checkout -- internal/api/v1/docs/docs.go

patch-epinio-deployment:
	@./scripts/patch-epinio-deployment.sh

appchart:
	@./scripts/appchart.sh

########################################################################
# Docs

getswagger:
	( [ -x "$$(command -v swagger)" ] || go install github.com/go-swagger/go-swagger/cmd/swagger@v0.28.0 )

swagger: getswagger
	swagger generate spec > docs/references/api/swagger.json
	swagger validate        docs/references/api/swagger.json

swagger-serve:
	@./scripts/swagger-serve.sh

########################################################################
# Support

tools-install:
	@./scripts/tools-install.sh

tools-versions:
	@./scripts/tools-versions.sh

########################################################################
# Kube dev environments

minikube-start:
	@./scripts/minikube-start.sh

minikube-delete:
	@./scripts/minikube-delete.sh

install-cert-manager:
	helm repo add cert-manager https://charts.jetstack.io
	helm repo update
	echo "Installing Cert Manager"
	helm upgrade --install cert-manager --create-namespace -n cert-manager \
		--set installCRDs=true \
		--set extraArgs[0]=--enable-certificate-owner-ref=true \
		cert-manager/cert-manager --version 1.8.2 \
		--wait

install-epinio-ui:
	@./scripts/install-epinio-ui.sh

install-rancher:
	@./scripts/install-rancher.sh

uninstall-rancher:
	helm uninstall -n cattle-system rancher --wait || true

install-upgrade-responder:
	@./scripts/install-upgrade-responder.sh

uninstall-upgrade-responder:
	helm uninstall -n epinio upgrade-responder --wait || true

prepare_environment_k3d: build-linux-amd64 build-images
	@./scripts/prepare-environment-k3d.sh

unprepare_environment_k3d:
	kubectl delete --ignore-not-found=true secret regcred
	helm uninstall epinio -n epinio --wait || true

# Generate tests description file
generate-acceptance-readme:
	@./scripts/generate-readme acceptance > acceptance/README.md
