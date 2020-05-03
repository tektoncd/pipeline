MODULE   = $(shell env GO111MODULE=on $(GO) list -m)
DATE    ?= $(shell date +%FT%T%z)
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || \
			cat $(CURDIR)/.version 2> /dev/null || echo v0)
PKGS     = $(or $(PKG),$(shell env GO111MODULE=on $(GO) list ./...))
TESTPKGS = $(shell env GO111MODULE=on $(GO) list -f \
			'{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' \
			$(PKGS))
BIN      = $(CURDIR)/.bin

GOLANGCI_VERSION = v1.24.0

GO           = go
TIMEOUT_UNIT = 5m
TIMEOUT_E2E  = 20m
V = 0
Q = $(if $(filter 1,$V),,@)
M = $(shell printf "\033[34;1m🐱\033[0m")

export GO111MODULE=on

.PHONY: all
all: fmt lint | $(BIN) ; $(info $(M) building executable…) @ ## Build program binary
	$Q $(GO) build \
		-tags release \
		-ldflags '-X $(MODULE)/cmd.Version=$(VERSION) -X $(MODULE)/cmd.BuildDate=$(DATE)' \
		-o $(BIN)/$(basename $(MODULE)) main.go

$(BIN):
	@mkdir -p $@
$(BIN)/%: | $(BIN) ; $(info $(M) building $(PACKAGE)…)
	$Q tmp=$$(mktemp -d); \
	   env GO111MODULE=off GOPATH=$$tmp GOBIN=$(BIN) $(GO) get $(PACKAGE) \
		|| ret=$$?; \
	   rm -rf $$tmp ; exit $$ret

KO = $(BIN)/ko
$(BIN)/ko: PACKAGE=github.com/google/ko/cmd/ko

.PHONY: apply
apply: | $(KO) ; $(info $(M) ko apply -f config/) @ ## Apply config to the current cluster
	$Q $(KO) apply --strict -f config

TEST_UNIT_TARGETS := test-unit-verbose test-unit-race
test-unit-verbose: ARGS=-v
test-unit-race:    ARGS=-race
$(TEST_UNIT_TARGETS): test-unit
.PHONY: $(TEST_UNIT_TARGETS) test-unit
test-unit: ## Run unit tests
	$(GO) test -timeout $(TIMEOUT_UNIT) $(ARGS) ./...

TEST_E2E_TARGETS := test-e2e-short test-e2e-verbose test-e2e-race
test-e2e-short:   ARGS=-short
test-e2e-verbose: ARGS=-v
test-e2e-race:    ARGS=-race
$(TEST_E2E_TARGETS): test-e2e
.PHONY: $(TEST_E2E_TARGETS) test-e2e
test-e2e:  ## Run end-to-end tests
	$(GO) test -timeout $(TIMEOUT_E2E) -tags e2e $(ARGS) ./test/...

.PHONY: test-yamls
test-yamls: ## Run yaml tests
	./test/e2e-tests-yaml.sh --run-tests

.PHONY: check tests
check tests: test-unit test-e2e test-yamls

RAM = $(BIN)/ram
$(BIN)/ram: PACKAGE=github.com/vdemeester/ram

.PHONY: watch-test
watch-test: | $(RAM) ; $(info $(M) watch and run tests) @ ## Watch and run tests
	$Q $(RAM) -- -failfast

.PHONY: watch-config
watch-config: | $(KO) ; $(info $(M) watch and apply config) @ ## Watch and apply to the current cluster
	$Q $(KO) apply --strict -W -f config

GOLINT = $(BIN)/golint
$(BIN)/golint: PACKAGE=golang.org/x/lint/golint

.PHONY: golint
golint: | $(GOLINT) ; $(info $(M) running golint…) @ ## Run golint
	$Q $(GOLINT) -set_exit_status $(PKGS)

GOLANGCILINT= $(BIN)/golangci-lint
$(BIN)/golangci-lint: ; $(info $(M) getting golangci-lint $(GOLANGCI_VERSION))
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(BIN) $(GOLANGCI_VERSION)

.PHONY: golangci-lint
golangci-lint: | $(GOLANGCILINT) ; $(info $(M) running golangci-lint) @ ## Run golangci-lint
	$Q $(GOLANGCILINT) run --modules-download-mode=vendor --max-issues-per-linter=0 --max-same-issues=0 --deadline 5m

.PHONY: fmt
fmt: ; $(info $(M) running gofmt…) @ ## Run gofmt on all source files
	$Q $(GO) fmt $(PKGS)
# Misc

.PHONY: clean
clean: ; $(info $(M) cleaning…)	@ ## Cleanup everything
	@rm -rf $(BIN)
	@rm -rf test/tests.* test/coverage.*

.PHONY: help
help:
	@grep -hE '^[ a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-17s\033[0m %s\n", $$1, $$2}'

.PHONY: version
version:
	@echo $(VERSION)
