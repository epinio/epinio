build: tools embed_files lint build-amd64

build-all: tools embed_files lint build-amd64 build-arm64 build-arm32 build-windows build-darwin

build-all-small:
	@$(MAKE) LDFLAGS+="-s -w" build-all

build-arm32: lint
	GOARCH="arm" GOOS="linux" go build -ldflags '$(LDFLAGS)' -o dist/carrier-linux-arm32

build-arm64: lint
	GOARCH="arm64" GOOS="linux" go build -ldflags '$(LDFLAGS)' -o dist/carrier-linux-arm64

build-amd64: lint
	GOARCH="amd64" GOOS="linux" go build -race -ldflags '$(LDFLAGS)' -o dist/carrier-linux-amd64

build-windows: lint
	GOARCH="amd64" GOOS="windows" go build -ldflags '$(LDFLAGS)' -o dist/carrier-windows-amd64

build-darwin: lint
	GOARCH="amd64" GOOS="darwin" go build -ldflags '$(LDFLAGS)' -o dist/carrier-darwin-amd64

test: lint
	ginkgo ./...

generate:
	go generate ./...

lint:	fmt vet tidy

vet:
	go vet ./...

tidy:
	go mod tidy

fmt:
	go fmt ./...

.PHONY: tools
tools:
	go get github.com/rakyll/statik

update_eirini:
	mkdir -p embedded-files/eirini
	wget https://github.com/cloudfoundry-incubator/eirini-release/releases/download/v2.0.0/eirini-yaml.tgz -O embedded-files/eirini/eirini-v2.0.0.tgz
	cp ../eirini/quarks-secrets.yaml embedded-files/eirini/

update_registry:
	helm package ../container-registry/chart/container-registry/ -d embedded-files

update_tekton:
	mkdir -p embedded-files/tekton
	wget https://storage.googleapis.com/tekton-releases/pipeline/previous/v0.19.0/release.yaml -O embedded-files/tekton/pipeline-v0.19.0.yaml
	wget https://storage.googleapis.com/tekton-releases/triggers/previous/v0.11.1/release.yaml -O embedded-files/tekton/triggers-v0.11.1.yaml
	wget https://github.com/tektoncd/dashboard/releases/download/v0.11.1/tekton-dashboard-release.yaml -O embedded-files/tekton/dashboard-v0.11.1.yaml
	cp ../tekton/admin-role.yaml embedded-files/tekton/
	cp ../tekton/staging.yaml embedded-files/tekton/
	cp ../tekton/triggers.yaml embedded-files/tekton/

embed_files:
	statik -m -f -src=./embedded-files

help:
	( echo _ _ ___ _____ ________ Overview ; carrier help ; for cmd in apps completion create-org delete help info install orgs push target uninstall ; do echo ; echo _ _ ___ _____ ________ Command $$cmd ; carrier $$cmd --help ; done ; echo ) | tee ../HELP
  