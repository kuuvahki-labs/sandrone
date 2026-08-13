.DEFAULT_GOAL := check

GO ?= go
GOFLAGS ?= -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls
PKGS ?= ./...
CMD_PKG ?= ./cmd/sandrone
BIN ?= sandrone
TMPDIR ?= /tmp
BUILD_BIN ?= $(TMPDIR)/sandrone-build
RULESET_CATALOG_DIR ?= $(CURDIR)/internal/service/catalog_builtin
GOLANGCI_LINT ?= golangci-lint
TESTFLAGS ?=
VERSION_FILE ?= $(CURDIR)/internal/buildinfo/VERSION
DEFAULT_VERSION := $(strip $(file <$(VERSION_FILE)))
ifeq ($(origin VERSION),undefined)
VERSION := $(DEFAULT_VERSION)
endif
REVISION_ORIGIN := $(origin REVISION)
LDFLAGS ?=
DOCKER ?= docker
SANDRONE_IMAGE ?= ghcr.io/kuuvahki-labs/sandrone:local

# Freeze raw command-line identity values as non-recursive variables. The
# validate-build-identity prerequisite checks them before any public target can
# expand them in a recipe, including on GNU Make 4.3.
override VERSION := $(value VERSION)
ifeq ($(REVISION_ORIGIN),undefined)
override REVISION := $(shell sh ./scripts/resolve-build-revision.sh)
else
override REVISION := $(value REVISION)
endif
BUILD_VERSION := $(value VERSION)

BUILD_REVISION := $(value REVISION)
ifeq ($(BUILD_REVISION),)
BUILD_VERSION := dev
endif
IMAGE_VERSION := $(if $(BUILD_REVISION),$(if $(BUILD_VERSION),$(BUILD_VERSION),$(DEFAULT_VERSION)),dev)

VERSION_X :=
ifneq ($(BUILD_VERSION),)
VERSION_X := -X github.com/kuuvahki-labs/sandrone/internal/buildinfo.rawVersion=$(BUILD_VERSION)
endif

REVISION_X :=
ifneq ($(BUILD_REVISION),)
REVISION_X := -X github.com/kuuvahki-labs/sandrone/internal/buildinfo.rawRevision=$(BUILD_REVISION)
endif

ifneq ($(strip $(BUILD_VERSION)$(BUILD_REVISION)),)
ifneq ($(findstring -ldflags,$(GOFLAGS)),)
$(error VERSION and REVISION cannot be combined with ldflags in GOFLAGS; move linker arguments to LDFLAGS)
endif
endif

BUILD_LDFLAGS := $(strip $(LDFLAGS) $(VERSION_X) $(REVISION_X))
BUILD_LDFLAGS_ARG :=
ifneq ($(BUILD_LDFLAGS),)
BUILD_LDFLAGS_ARG := -ldflags "$(BUILD_LDFLAGS)"
endif
BUILD_VCS_ARG := $(if $(BUILD_REVISION),,-buildvcs=false)

VALIDATED_TARGETS := help check ci fmt fmt-check vet test test-webui test-webui-e2e build build-bin build-check build-webui image lint ruleset-catalog release-artifacts snapshot-artifacts

.PHONY: validate-build-identity $(VALIDATED_TARGETS)

$(VALIDATED_TARGETS): | validate-build-identity

validate-build-identity: override export VERSION := $(value VERSION)
validate-build-identity: override export REVISION := $(value REVISION)
validate-build-identity:
	@if [ "$$(sh ./scripts/validate-build-version.sh)" != "ok" ]; then \
		printf '%s\n' 'VERSION must be empty or contain only ASCII letters, digits, dots, plus signs, and hyphens' >&2; \
		exit 2; \
	fi
	@if [ "$$(sh ./scripts/validate-build-revision.sh)" != "ok" ]; then \
		printf '%s\n' 'REVISION must be empty or a complete 40- or 64-character hexadecimal Git object ID' >&2; \
		exit 2; \
	fi

help: ## Show available targets.
	@printf '%s\n' 'Common targets:'
	@printf '  %-28s %s\n' 'check' 'Run fmt-check, vet, test, and build.'
	@printf '  %-28s %s\n' 'ci' 'Run check and lint.'
	@printf '  %-28s %s\n' 'fmt' 'Format Go files with go fmt.'
	@printf '  %-28s %s\n' 'fmt-check' 'Check Go formatting without changing files.'
	@printf '  %-28s %s\n' 'vet' 'Run go vet.'
	@printf '  %-28s %s\n' 'test' 'Run default Go tests.'
	@printf '  %-28s %s\n' 'test-webui' 'Run Web UI unit tests once.'
	@printf '  %-28s %s\n' 'test-webui-e2e' 'Run Web UI Playwright smoke tests.'
	@printf '  %-28s %s\n' 'build' 'Generate the rule-set catalog and build the CLI to BUILD_BIN.'
	@printf '  %-28s %s\n' 'build-bin' 'Generate the catalog and build the CLI in the repo; override BIN as needed.'
	@printf '  %-28s %s\n' 'release-artifacts' 'Build Linux release archives and checksums.'
	@printf '  %-28s %s\n' 'snapshot-artifacts' 'Build local dev Linux archives under dist/snapshot.'
	@printf '  %-28s %s\n' 'ruleset-catalog' 'Generate the ignored build-time rule-set URL snapshot.'
	@printf '  %-28s %s\n' 'build-webui' 'Build web UI assets and copy them into Go embed static files.'
	@printf '  %-28s %s\n' 'image' 'Build a locally tagged container image.'
	@printf '  %-28s %s\n' 'lint' 'Run golangci-lint with .golangci.yml.'

check: fmt-check vet test build-check

ci: check lint

fmt:
	$(GO) fmt $(PKGS)

fmt-check:
	@test -z "$$(find . -type f -name '*.go' -not -path './.git/*' -not -path './.claude/*' -exec gofmt -l {} +)"

vet:
	$(GO) vet $(GOFLAGS) $(PKGS)

test:
	$(GO) test $(GOFLAGS) $(TESTFLAGS) $(PKGS)

test-webui:
	cd web && pnpm test:run

test-webui-e2e:
	cd web && pnpm test:e2e

ruleset-catalog:
	GO="$(GO)" GOFLAGS="-mod=readonly" ./scripts/generate-ruleset-catalog.sh "$(RULESET_CATALOG_DIR)"

build: ruleset-catalog
	$(GO) build $(GOFLAGS) $(BUILD_VCS_ARG) $(BUILD_LDFLAGS_ARG) -o $(BUILD_BIN) $(CMD_PKG)

build-bin: ruleset-catalog
	$(GO) build $(GOFLAGS) $(BUILD_VCS_ARG) $(BUILD_LDFLAGS_ARG) -o $(BIN) $(CMD_PKG)

build-check:
	$(GO) build $(GOFLAGS) $(BUILD_VCS_ARG) $(BUILD_LDFLAGS_ARG) -o $(BUILD_BIN) $(CMD_PKG)

release-artifacts: build-webui
	ARTIFACT_KIND=release VERSION="$(BUILD_VERSION)" REVISION="$(BUILD_REVISION)" ./scripts/build-release-artifacts.sh

snapshot-artifacts: build-webui
	ARTIFACT_KIND=snapshot VERSION=dev REVISION="" OUTPUT_DIR="$(CURDIR)/dist/snapshot" ./scripts/build-release-artifacts.sh

build-webui:
	./scripts/build-webui.sh

image:
	$(DOCKER) build \
		--build-arg VERSION="$(IMAGE_VERSION)" \
		--build-arg REVISION="$(BUILD_REVISION)" \
		--label org.opencontainers.image.version="$(IMAGE_VERSION)" \
		--label org.opencontainers.image.revision="$(BUILD_REVISION)" \
		--tag "$(SANDRONE_IMAGE)" \
		.

lint:
	$(GOLANGCI_LINT) run
