all: bin/tkn test

FORCE:

bin/%: cmd/% FORCE
	@go build -v -o $@$(EXEC_EXT) ./$<

check: lint test

test: test-unit ## run all tests

lint: ## run linter(s)
	@echo "Linting..."
	@golangci-lint run ./...

test-unit: ## run unit tests
	@echo "Running unit tests..."
	@go test -v -cover ./...
docs: bin/docs ## update docs
	@echo "Update generated docs"
	@./bin/docs --target=./docs/cmd
