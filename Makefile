.DEFAULT_GOAL := help

# Disable/enable various make features.
#
# https://www.gnu.org/software/make/manual/html_node/Options-Summary.html
MAKEFLAGS += --no-builtin-rules
MAKEFLAGS += --no-builtin-variables
MAKEFLAGS += --no-print-directory
MAKEFLAGS += --warn-undefined-variables

# This makes all targets silent by default, unless VERBOSE is set.
ifndef VERBOSE
.SILENT:
endif

TAG 	 ?= local
GO_FLAGS = -tags netgo -mod=vendor

##@ Build

test:: ## Run unit-tests
	go test -v $(GO_FLAGS) ./...

build:: ## Build binary for the current platform
	go vet
	go fmt
	go build $(GO_FLAGS)

##@ Release

release:: clean
	git tag -a $(TAG) -m "$(TAG)"
	git push origin $(TAG)
	docker build -t hendrikmaus/openhab-auth-router:$(TAG) .
	docker build -f Dockerfile.arm32v6 -t hendrikmaus/openhab-auth-router:$(TAG)-arm32v6 .
	docker push hendrikmaus/openhab-auth-router:$(TAG)
	docker push hendrikmaus/openhab-auth-router:$(TAG)-arm32v6
	goreleaser

##@ Helpers

clean:: ## Clean working directory
	-rm ./openhab-auth-router ./openhab-auth-router.exe ./dist
	-mkdir -p ./dist

help::  ## Display this help
	awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[0-9a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
