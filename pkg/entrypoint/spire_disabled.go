//go:build disable_spire

/*
Copyright 2025 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package entrypoint

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/result"
)

// EntrypointerAPIClient defines the interface for SPIRE operations
type EntrypointerAPIClient interface {
	Sign(ctx context.Context, results []result.RunResult) ([]result.RunResult, error)
}

func signResults(ctx context.Context, api EntrypointerAPIClient, results []result.RunResult) ([]result.RunResult, error) {
	return nil, nil
}
