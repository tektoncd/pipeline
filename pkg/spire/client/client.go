/*
Copyright 2026 The Tekton Authors

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

// Package client defines the SPIRE workload-signing client contract.
package client

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/result"
)

// EntrypointerAPIClient signs results using the workload identity.
type EntrypointerAPIClient interface {
	Close() error
	// Sign returns the signature material to append to the output results.
	Sign(ctx context.Context, results []result.RunResult) ([]result.RunResult, error)
}
