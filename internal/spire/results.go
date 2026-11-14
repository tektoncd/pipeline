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

package spire

import (
	"fmt"
	"sort"
	"strings"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1/types"
	"github.com/tektoncd/pipeline/pkg/result"
)

const (
	// KeySVID identifies the TaskRun SVID.
	KeySVID = "SVID"
	// KeySignatureSuffix identifies result signatures.
	KeySignatureSuffix = ".sig"
	// KeyResultManifest identifies the signed result manifest.
	KeyResultManifest = "RESULT_MANIFEST"
)

// ResultValue returns the canonical value used to sign and verify a result.
func ResultValue(result result.RunResult) (string, error) {
	aos := v1.ParamValue{}
	err := aos.UnmarshalJSON([]byte(result.Value))
	valList := []string{}
	if err != nil {
		return "", fmt.Errorf("unmarshal error for key: %s", result.Key)
	}
	switch aos.Type {
	case v1.ParamTypeString:
		return aos.StringVal, nil
	case v1.ParamTypeArray:
		valList = append(valList, aos.ArrayVal...)
		return strings.Join(valList, ","), nil
	case v1.ParamTypeObject:
		keys := make([]string, 0, len(aos.ObjectVal))
		for k := range aos.ObjectVal {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			valList = append(valList, k)
			valList = append(valList, aos.ObjectVal[k])
		}
		return strings.Join(valList, ","), nil
	}
	return "", fmt.Errorf("invalid result type for key: %s", result.Key)
}
