SHELL := /bin/bash
BINARY ?= vistan
BUMP ?= patch
VERSION ?=

LDFLAGS := $(if $(VERSION),-X github.com/zovaris/vistan/internal/vistan.version=$(VERSION))

.PHONY: help build install test fmt vet lint check next-version version clean

help: ## Show this help
	@awk 'BEGIN { FS = ":.*##" ; printf "\nvistan — make targets\n\n" } \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  %-14s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## Build the binary into ./vistan
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/vistan

install: ## go install ./cmd/vistan into $GOBIN
	go install -trimpath -ldflags "$(LDFLAGS)" ./cmd/vistan

test: ## Run the tests
	go test ./...

fmt: ## Fail if a file is not gofmt'ed
	@test -z "$$(gofmt -l .)" || { gofmt -l .; echo "run gofmt -w ."; exit 1; }

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

check: fmt vet test ## What CI and the release run

next-version: ## Print the next version and tag without changing anything
	@./tools/version.sh --dry-run $(if $(VERSION),--version $(VERSION),--bump $(BUMP))

version: ## Tag a release and push it (VERSION=x.y.z or BUMP=patch|minor|major; DRY=1, NO_PUSH=1)
	@./tools/version.sh $(if $(VERSION),--version $(VERSION),--bump $(BUMP)) $(if $(DRY),--dry-run) $(if $(NO_PUSH),--no-push)

clean: ## Remove the build output
	rm -rf $(BINARY) dist
