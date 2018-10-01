/*
Copyright 2018 The Knative Authors

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

package duck

import (
	"encoding/json"

	"github.com/mattbaird/jsonpatch"
)

func CreatePatch(before, after interface{}) (JSONPatch, error) {
	// Marshal the before and after.
	rawBefore, err := json.Marshal(before)
	if err != nil {
		return nil, err
	}

	rawAfter, err := json.Marshal(after)
	if err != nil {
		return nil, err
	}

	return jsonpatch.CreatePatch(rawBefore, rawAfter)
}

type JSONPatch []jsonpatch.JsonPatchOperation

func (p JSONPatch) MarshalJSON() ([]byte, error) {
	return json.Marshal([]jsonpatch.JsonPatchOperation(p))
}
