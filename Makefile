########################################################################
# Development

tools-install:
	@./scripts/tools-install.sh

tools-versions:
	@./scripts/tools-versions.sh

version:
	@./scripts/version.sh

lint: shellcheck yamllint helmlint httplint

helmlint:
	@./scripts/helmlint.sh

shellcheck:
	@./scripts/shellcheck.sh

yamllint:
	@./scripts/yamllint.sh

.PHONY: httplint
httplint:
	@./src/kubecf-tools/httplint/httplint.sh
