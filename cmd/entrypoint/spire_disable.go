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

package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/tektoncd/pipeline/pkg/result"
)

var (
	enableSpire = flag.Bool("enable_spire", false, "If specified by configmap, this enables spire signing and verification")
)

// EntrypointerAPIClient interface maps to the spire entrypointer API to interact with spire
type EntrypointerAPIClient interface {
	Close() error
	// Sign returns the signature material to be put in the RunResult to append to the output results
	Sign(ctx context.Context, results []result.RunResult) ([]result.RunResult, error)
}

func initializeSpireAPI() EntrypointerAPIClient {
	if enableSpire != nil && *enableSpire {
		log.Fatal("Error: SPIRE is disabled in this build, but enableSpire was set to true. Please recompile with SPIRE support.")
		os.Exit(1)
	}
	return nil
}
