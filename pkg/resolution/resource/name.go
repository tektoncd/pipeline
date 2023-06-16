/*
Copyright 2022 The Tekton Authors

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

package resource

import (
	"fmt"
	"hash"
	"hash/fnv"
	"sort"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// nameHasher returns the hash.Hash to use when generating names.
func nameHasher() hash.Hash {
	return fnv.New128a()
}

// GenerateDeterministicName makes a best-effort attempt to create a
// unique but reproducible name for use in a Request. The returned value
// will have the format {prefix}-{hash} where {prefix} is
// given and {hash} is nameHasher(base) + nameHasher(param1) +
// nameHasher(param2) + ...
func GenerateDeterministicName(prefix, base string, params v1.Params) (string, error) {
	hasher := nameHasher()
	if _, err := hasher.Write([]byte(base)); err != nil {
		return "", err
	}

	sortedParams := make(v1.Params, len(params))
	for i := range params {
		sortedParams[i] = *params[i].DeepCopy()
	}
	sort.SliceStable(sortedParams, func(i, j int) bool {
		return sortedParams[i].Name < sortedParams[j].Name
	})
	for _, p := range sortedParams {
		if _, err := hasher.Write([]byte(p.Name)); err != nil {
			return "", err
		}
		switch p.Value.Type {
		case v1.ParamTypeString:
			if _, err := hasher.Write([]byte(p.Value.StringVal)); err != nil {
				return "", err
			}
		case v1.ParamTypeArray, v1.ParamTypeObject:
			asJSON, err := p.Value.MarshalJSON()
			if err != nil {
				return "", err
			}
			if _, err := hasher.Write(asJSON); err != nil {
				return "", err
			}
		}
	}
	return fmt.Sprintf("%s-%x", prefix, hasher.Sum(nil)), nil
}
