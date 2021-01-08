build: fmt vet
	go build -o dist/carrier

test: fmt vet
	ginkgo ./...

generate_fakes:
	go generate ./...

vet:
	go vet ./...

fmt:
	go fmt ./...
