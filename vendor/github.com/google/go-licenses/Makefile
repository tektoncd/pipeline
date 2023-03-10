# Copyright 2022 Google Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# All of these should pass before sending a PR.
precommit: test lint tidy

test: FORCE
	go test ./...

# Note, when upgrading, also upgrade version in .github/workflows/golangci-lint.yml.
GOLANGCI_LINT_VERSION=v1.49
lint:  FORCE
	@which golangci-lint >/dev/null || ( \
		echo 'golangci-lint is not installed. Install by:\ngo install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)' \
		&& exit 1 )
	golangci-lint run -v

tidy: FORCE
	go mod tidy

FORCE: ;
