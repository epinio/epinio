
ARM32	=	dist/arm32
ARM64	=	dist/arm64
AMD64	=	dist/amd64

build: lint build-amd64 build-arm64 build-arm32

build-arm32: lint
	mkdir -p ${ARM32}
	GOARCH="arm" go build -v -o ${ARM32}/carrier

build-arm64: lint
	mkdir -p ${ARM64}
	GOARCH="arm64" go build -v -o ${ARM64}/carrier

build-amd64: lint
	mkdir -p ${AMD64}
	GOARCH="amd64" go build -v -o ${AMD64}/carrier

test: lint
	ginkgo ./...

generate_fakes:
	go generate ./...

lint:	fmt vet

vet:
	go vet ./...

fmt:
	go fmt ./...
