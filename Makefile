########################################################################
# Development

tools-install:
	@./scripts/tools-install.sh

tools-versions:
	@./scripts/tools-versions.sh

version:
	@./scripts/version.sh

help:
	@$(MAKE) -C cli help

build:
	@$(MAKE) -C cli build

build-all:
	@$(MAKE) -C cli build-all

build-all-small:
	@$(MAKE) -C cli build-all-small

build-arm32:
	@$(MAKE) -C cli build-arm32

build-arm64:
	@$(MAKE) -C cli build-arm64

build-amd64:
	@$(MAKE) -C cli build-amd64

build-windows:
	@$(MAKE) -C cli build-windows

build-darwin:
	@$(MAKE) -C cli build-darwin

compress:
	upx --brute -1 ./cli/dist/carrier-linux-arm32
	upx --brute -1 ./cli/dist/carrier-linux-arm64
	upx --brute -1 ./cli/dist/carrier-linux-amd64
	upx --brute -1 ./cli/dist/carrier-windows-amd64
	upx --brute -1 ./cli/dist/carrier-darwin-amd64

test:
	@$(MAKE) -C cli test

lint:
	@$(MAKE) -C cli lint

fmt:
	@$(MAKE) -C cli fmt

vet:
	@$(MAKE) -C cli vet

tidy:
	@$(MAKE) -C cli tidy

generate:
	@$(MAKE) -C cli generate

########################################################################
# Kube dev environments

minikube-start:
	@./scripts/minikube-start.sh

minikube-delete:
	@./scripts/minikube-delete.sh


# lint: shellcheck yamllint helmlint httplint

# helmlint:
# 	@./scripts/helmlint.sh

# shellcheck:
# 	@./scripts/shellcheck.sh

# yamllint:
# 	@./scripts/yamllint.sh

# .PHONY: httplint
# httplint:
# 	@./src/kubecf-tools/httplint/httplint.sh
