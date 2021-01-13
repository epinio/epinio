build: tools embed_files lint build-amd64

build-all: tools embed_files lint build-amd64 build-arm64 build-arm32 build-windows build-darwin

build-arm32: lint
	GOARCH="arm" GOOS="linux" go build -o dist/carrier-linux-arm32

build-arm64: lint
	GOARCH="arm64" GOOS="linux" go build -o dist/carrier-linux-arm64

build-amd64: lint
	GOARCH="amd64" GOOS="linux" go build -o dist/carrier-linux-amd64

build-windows: lint
	GOARCH="amd64" GOOS="windows" go build -o dist/carrier-windows-amd64

build-darwin: lint
	GOARCH="amd64" GOOS="darwin" go build -o dist/carrier-darwin-amd64

test: lint
	ginkgo ./...

generate_fakes:
	go generate ./...

lint:	fmt vet

vet:
	go vet ./...

fmt:
	go fmt ./...

.PHONY: tools
tools:
	go get github.com/rakyll/statik

package_registry:
	helm package ../container-registry/chart/container-registry/ -d embedded-files

embed_files:
	statik -f -src=./embedded-files
