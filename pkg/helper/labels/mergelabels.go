// Copyright © 2019 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package labels

import (
	"errors"
	"strings"
)

const invalidLabel = "invalid input format for label parameter: "

func MergeLabels(l map[string]string, optLabel []string) (map[string]string, error) {
	labels, err := parseLabels(optLabel)
	if err != nil {
		return nil, err
	}
	if len(labels) == 0 {
		return l, nil
	}

	if l == nil {
		return labels, nil
	}

	for k, v := range labels {
		l[k] = v
	}
	return l, nil
}

func parseLabels(p []string) (map[string]string, error) {
	labels := map[string]string{}
	for _, v := range p {
		r := strings.SplitN(v, "=", 2)
		if len(r) != 2 {
			return nil, errors.New(invalidLabel + v)
		}
		labels[r[0]] = r[1]
	}
	return labels, nil
}
