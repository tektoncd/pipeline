all: bin/tkn test

FORCE:

./vendor: go.mod go.sum
	@go mod vendor

.PHONY: cross
cross: amd64 386 arm arm64 ## build cross platform binaries

.PHONY: amd64
amd64:
	GOOS=linux GOARCH=amd64 go build -o bin/tkn-linux-amd64 ./cmd/tkn
	GOOS=windows GOARCH=amd64 go build -o bin/tkn-windows-amd64 ./cmd/tkn
	GOOS=darwin GOARCH=amd64 go build -o bin/tkn-darwin-amd64 ./cmd/tkn

.PHONY: 386
386:
	GOOS=linux GOARCH=386 go build -o bin/tkn-linux-386 ./cmd/tkn
	GOOS=windows GOARCH=386 go build -o bin/tkn-windows-386 ./cmd/tkn
	GOOS=darwin GOARCH=386 go build -o bin/tkn-darwin-386 ./cmd/tkn

.PHONY: arm
arm:
	GOOS=linux GOARCH=arm go build -o bin/tkn-linux-arm ./cmd/tkn
	GOOS=windows GOARCH=arm go build -o bin/tkn-windows-arm ./cmd/tkn

.PHONY: arm64
arm64:
	GOOS=linux GOARCH=arm64 go build -o bin/tkn-linux-arm64 ./cmd/tkn

bin/%: cmd/% ./vendor FORCE
	@go build -v -o $@ ./$<

check: lint test

.PHONY: test
test: test-unit ## run all tests

.PHONY: lint
lint: ## run linter(s)
	@echo "Linting..."
	@golangci-lint run ./...

.PHONY: test-unit
test-unit: ./vendor ## run unit tests
	@echo "Running unit tests..."
	@go test -v -cover ./...

.PHONY: test-e2e
test-e2e: ./vendor ## run e2e tests
	@echo "Running e2e tests..."
	@LOCAL_CI_RUN=true bash ./test/e2e-tests.sh

.PHONY: docs
docs: bin/docs ## update docs
	@echo "Update generated docs"
	@./bin/docs --target=./docs/cmd

.PHONY: man
man: bin/docs ## update manpages
	@echo "Update generated manpages"
	@./bin/docs --target=./docs/man/man1 --kind=man

.PHONY: clean
clean: ## clean build artifacts
	rm -fR bin

.PHONY: help
help: ## print this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {gsub("\\\\n",sprintf("\n%22c",""), $$2);printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
