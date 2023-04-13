/*
Copyright 2019 The Tekton Authors

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

package termination

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/tektoncd/pipeline/pkg/result"
	"go.uber.org/zap"
)

// ParseMessage parses a termination message as results.
//
// If more than one item has the same key, only the latest is returned. Items
// are sorted by their key.
func ParseMessage(logger *zap.SugaredLogger, msg string) ([]result.RunResult, error) {
	if msg == "" {
		return nil, nil
	}

	var r []result.RunResult
	if err := json.Unmarshal([]byte(msg), &r); err != nil {
		return nil, fmt.Errorf("parsing message json: %w, msg: %s", err, msg)
	}

	for i, rr := range r {
		if rr == (result.RunResult{}) {
			// Erase incorrect result
			r[i] = r[len(r)-1]
			r = r[:len(r)-1]
			logger.Errorf("termination message contains non taskrun or pipelineresource result keys")
		}
	}

	// Remove duplicates (last one wins) and sort by key.
	m := map[string]result.RunResult{}
	for _, rr := range r {
		m[rr.Key] = rr
	}
	r2 := make([]result.RunResult, 0, len(m))
	for _, v := range m {
		r2 = append(r2, v)
	}
	sort.Slice(r2, func(i, j int) bool { return r2[i].Key < r2[j].Key })

	return r2, nil
}
