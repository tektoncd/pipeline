all: bin/tkn test

bin/%: cmd/%
	@go build -v -o $@$(EXEC_EXT) ./$<

check: lint test

test: test-unit ## run all tests

lint: ## run linter(s)
	@echo "Linting..."
	@golangci-lint run ./...

test-unit: ## run unit tests
	@echo "Running unit tests..."
	@go test -v -cover ./...
