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

	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

// ParseMessage parses a termination message as results.
//
// If more than one item has the same key, only the latest is returned. Items
// are sorted by their key.
func ParseMessage(msg string) ([]v1alpha1.PipelineResourceResult, error) {
	if msg == "" {
		return nil, nil
	}
	var r []v1alpha1.PipelineResourceResult
	if err := json.Unmarshal([]byte(msg), &r); err != nil {
		return nil, fmt.Errorf("parsing message json: %v", err)
	}

	// Remove duplicates (last one wins) and sort by key.
	m := map[string]v1alpha1.PipelineResourceResult{}
	for _, rr := range r {
		m[rr.Key] = rr
	}
	var r2 []v1alpha1.PipelineResourceResult
	for _, v := range m {
		r2 = append(r2, v)
	}
	sort.Slice(r2, func(i, j int) bool { return r2[i].Key < r2[j].Key })

	return r2, nil
}
