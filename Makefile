.DEFAULT_GOAL := check

GO ?= go
GOFLAGS ?= -mod=readonly -tags probe_singbox
PKGS ?= ./...
CMD_PKG ?= ./cmd/sandrone
BIN ?= sandrone
TMPDIR ?= /tmp
BUILD_BIN ?= $(TMPDIR)/sandrone-build
RULESET_CATALOG_DIR ?= $(CURDIR)/internal/service/catalog_builtin
GOLANGCI_LINT ?= golangci-lint
TESTFLAGS ?=
VERSION ?=
LDFLAGS ?=

# Validate command-line VERSION before storing or expanding it anywhere else. The
# value function freezes the raw command-line text as a non-recursive variable;
# command-line variables are exported by Make without recipe interpolation.
override VERSION := $(value VERSION)
export VERSION
VERSION_VALIDATION := $(shell sh ./scripts/validate-build-version.sh)
unexport VERSION
ifneq ($(VERSION_VALIDATION),ok)
$(error VERSION must be empty or contain only ASCII letters, digits, dots, plus signs, and hyphens)
endif
BUILD_VERSION := $(value VERSION)

VERSION_X :=
ifneq ($(BUILD_VERSION),)
ifneq ($(findstring -ldflags,$(GOFLAGS)),)
$(error VERSION cannot be combined with ldflags in GOFLAGS; move linker arguments to LDFLAGS)
endif
VERSION_X := -X github.com/kuuvahki-labs/sandrone/internal/buildinfo.rawVersion=$(BUILD_VERSION)
endif

BUILD_LDFLAGS := $(strip $(LDFLAGS) $(VERSION_X))
BUILD_LDFLAGS_ARG :=
ifneq ($(BUILD_LDFLAGS),)
BUILD_LDFLAGS_ARG := -ldflags "$(BUILD_LDFLAGS)"
endif

.PHONY: help check ci fmt fmt-check vet test test-webui test-webui-e2e build build-bin build-check build-webui lint ruleset-catalog \
	test-probe test-probe-mihomo test-probe-singbox \
	build-probe-mihomo build-probe-singbox

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
	@printf '  %-28s %s\n' 'ruleset-catalog' 'Generate the ignored build-time rule-set URL snapshot.'
	@printf '  %-28s %s\n' 'build-webui' 'Build web UI assets and copy them into Go embed static files.'
	@printf '  %-28s %s\n' 'lint' 'Run golangci-lint with .golangci.yml.'
	@printf '  %-28s %s\n' 'test-probe' 'Run mihomo and sing-box probe build tag tests.'
	@printf '  %-28s %s\n' 'test-probe-mihomo' 'Run probe tests with -tags probe_mihomo.'
	@printf '  %-28s %s\n' 'test-probe-singbox' 'Run probe tests with -tags probe_singbox.'
	@printf '  %-28s %s\n' 'build-probe-mihomo' 'Build CLI with -tags probe_mihomo.'
	@printf '  %-28s %s\n' 'build-probe-singbox' 'Build CLI with -tags probe_singbox.'

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
	$(GO) build $(GOFLAGS) $(BUILD_LDFLAGS_ARG) -o $(BUILD_BIN) $(CMD_PKG)

build-bin: ruleset-catalog
	$(GO) build $(GOFLAGS) $(BUILD_LDFLAGS_ARG) -o $(BIN) $(CMD_PKG)

build-check:
	$(GO) build $(GOFLAGS) $(BUILD_LDFLAGS_ARG) -o $(BUILD_BIN) $(CMD_PKG)

build-webui:
	./scripts/build-webui.sh

lint:
	$(GOLANGCI_LINT) run

test-probe: test-probe-mihomo test-probe-singbox

test-probe-mihomo:
	$(GO) test $(GOFLAGS) $(TESTFLAGS) -tags probe_mihomo ./internal/probe ./internal/service

test-probe-singbox:
	$(GO) test $(GOFLAGS) $(TESTFLAGS) -tags probe_singbox ./internal/probe ./internal/service

build-probe-mihomo:
	$(GO) build $(GOFLAGS) $(BUILD_LDFLAGS_ARG) -tags probe_mihomo $(CMD_PKG)

build-probe-singbox:
	$(GO) build $(GOFLAGS) $(BUILD_LDFLAGS_ARG) -tags probe_singbox $(CMD_PKG)
