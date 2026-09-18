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

package entrypoint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Only fingerprints cross the step boundary, not the accumulated result payload.
// Each step writes its own metadata volume; later steps mount that volume read-only.
// Comparing reported values also preserves pre-populated result files and allows
// steps to read, append to, or overwrite results in the shared results directory.
type taskResultHashes map[string]string

const taskResultHashesFile = "task-results.json"

func (e Entrypointer) tracksTaskResults() bool {
	return e.ResultExtractionMethod == ResultExtractionMethodTerminationMessage &&
		len(e.Results) > 0 && e.Results[0] != "" && e.StepMetadataDir != ""
}

func readTaskResultHashes(dir string) (taskResultHashes, error) {
	hashes := taskResultHashes{}
	if dir == "" {
		return hashes, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, taskResultHashesFile))
	if os.IsNotExist(err) {
		return hashes, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read previous task result fingerprints: %w", err)
	}
	if err := json.Unmarshal(data, &hashes); err != nil {
		return nil, fmt.Errorf("parse previous task result fingerprints: %w", err)
	}
	if hashes == nil {
		hashes = taskResultHashes{}
	}
	return hashes, nil
}

func (h taskResultHashes) write(dir string) error {
	data, err := json.Marshal(h)
	if err != nil {
		return fmt.Errorf("encode task result fingerprints: %w", err)
	}
	// Later steps may run as different UIDs and must be able to read this file.
	// Their mounts of this step's runtime volume are read-only.
	if err := os.WriteFile(filepath.Join(dir, taskResultHashesFile), data, 0o644); err != nil { // #nosec G306
		return fmt.Errorf("write task result fingerprints: %w", err)
	}
	return nil
}
