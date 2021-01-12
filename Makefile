########################################################################
# Development

tools-install:
	@./scripts/tools-install.sh

tools-versions:
	@./scripts/tools-versions.sh

version:
	@./scripts/version.sh

build:
	@$(MAKE) -C cli build

build-arm32:
	@$(MAKE) -C cli build-arm32

build-arm64:
	@$(MAKE) -C cli build-arm64

build-amd64:
	@$(MAKE) -C cli build-amd64

test:
	@$(MAKE) -C cli test

lint:
	@$(MAKE) -C cli lint

fmt:
	@$(MAKE) -C cli fmt

vet:
	@$(MAKE) -C cli vet

generate_fakes:
	@$(MAKE) -C cli generate_fakes



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
