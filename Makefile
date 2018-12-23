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

IMAGE_TAG 	?= local
GO_FLAGS 		= -tags netgo -ldflags "-X 'main.Version=$(IMAGE_TAG)'" -mod=vendor
GO_ENV 			:= GO111MODULE="on"
GO_BIN      ?= go
GO_COMMIT   ?= ee55f0856a3f1fed5d8c15af54c40e4799c2d32f

##@ Build

test: ## Run unit-tests
	@$(GO_ENV) $(GO_BIN) test -v $(GO_FLAGS) ./...
.PHONY: test

build: ## Build binary for the current platform
	$(GO_ENV) $(GO_BIN) build $(GO_FLAGS)
.PHONY: build

build-golang: ## Build base image to compile the app
	docker build -t hendrikmaus/golang:$(GO_COMMIT) --build-arg GO_COMMIT=$(GO_COMMIT) -f Dockerfile.golang .
.PHONY: build-golang

##@ Packaging

pkg-all: pkg-freebsd pkg-linux pkg-mac pkg-arm32v6 pkg-win pkg-docker pkg-docker-arm32v6 ## Build all distribution files (docker imahe not pushed)

pkg-freebsd: ## Build openhab-auth-router zip-file for FreeBSD (x64)
	echo "\033[0;33mBuilding for FreeBSD/x64\033[0;0m"
	GOOS=freebsd GOARCH=amd64 $(GO_ENV) $(GO_BIN) build $(GO_FLAGS)
	zip dist/openhab-auth-router-$(IMAGE_TAG)-FreeBSD_x64.zip openhab-auth-router
.PHONY: pkg-freebsd

pkg-linux: ## Build openhab-auth-router zip-file for Linux (x64)
	echo "\033[0;33mBuilding for Linux/x64\033[0;0m"
	GOOS=linux GOARCH=amd64 $(GO_ENV) $(GO_BIN) build $(GO_FLAGS)
	zip dist/openhab-auth-router-$(IMAGE_TAG)-Linux_x64.zip openhab-auth-router
.PHONY: pkg-linux

pkg-mac: ## Build openhab-auth-router zip-file for MacOS X (x64)
	echo "\033[0;33mBuilding for MacOS X (MacOS/x64)\033[0;0m"
	GOOS=darwin GOARCH=amd64 $(GO_ENV) $(GO_BIN) build $(GO_FLAGS)
	zip dist/openhab-auth-router-$(IMAGE_TAG)-MacOS_x64.zip openhab-auth-router
.PHONY: pkg-mac

pkg-arm32v6: ## Build openhab-auth-router zip-file for Raspberry Pi / Linux (ARM32v6)
	echo "\033[0;33mBuilding for Raspberry Pi (Linux/ARM32v6)\033[0;0m"
	GOOS=linux GOARCH=arm GOARM=6 $(GO_ENV) $(GO_BIN) build $(GO_FLAGS)
	zip dist/openhab-auth-router-$(IMAGE_TAG)-Linux_Arm6.zip openhab-auth-router
.PHONY: pkg-arm32v6

pkg-win: ## Build openhab-auth-router zip-file for Windows (x64)
	echo "\033[0;33mBuilding for Windows/x64\033[0;0m"
	GOOS=windows GOARCH=amd64 $(GO_ENV) $(GO_BIN) build $(GO_FLAGS)
	zip dist/openhab-auth-router-$(IMAGE_TAG)-Windows_x64.zip openhab-auth-router.exe
	rm openhab-auth-router.exe
.PHONY: pkg-win

pkg-docker: ## Build openhab-auth-router docker image
	echo "\033[0;33mBuilding docker image\033[0;0m"
	docker build --build-arg GO_COMMIT=$(GO_COMMIT) -t hendrikmaus/openhab-auth-router:$(IMAGE_TAG) .
.PHONY: pkg-docker

pkg-docker-arm32v6: ## Build openhab-auth-router docker image for arm32v6
	echo "\033[0;33mBuilding docker image for arm32v6\033[0;0m"
	docker build -f Dockerfile.arm32v6 --build-arg GO_COMMIT=$(GO_COMMIT) -t hendrikmaus/openhab-auth-router:$(IMAGE_TAG)-arm32v6 .
.PHONY: pkg-docker-arm32v6

pkg-docker-push: ## Push openhab-auth-router docker images
	docker push hendrikmaus/openhab-auth-router:$(IMAGE_TAG)
	docker push hendrikmaus/openhab-auth-router:$(IMAGE_TAG)-arm32v6
.PHONY: pkg-docker-push

##@ Dependencies

vendor: ## Generate the vendor folder
	$(GO_ENV) $(GO_BIN) mod vendor
.PHONY: vendor

tidy: ## Ensure dependencies are up2date
	$(GO_ENV) $(GO_BIN) mod tidy
.PHONY: vendor

##@ Helpers

clean: ## Clean working directory
	-rm ./openhab-auth-router ./openhab-auth-router.exe ./dist
	-mkdir -p ./dist
.PHONY: clean

help:  ## Display this help
	awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[0-9a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
