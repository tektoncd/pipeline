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

package buildtest

import (
	"encoding/json"

	"github.com/go-yaml/yaml"
)

// From: https://stackoverflow.com/questions/40737122/convert-yaml-to-json-without-struct-golang
func convert(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[k.(string)] = convert(v)
		}
		return m2
	case []interface{}:
		for i, v := range x {
			x[i] = convert(v)
		}
	}
	return i
}

// YAMLToJSON converts the given YAML bytes to JSON bytes.
func YAMLToJSON(y []byte) ([]byte, error) {
	var body interface{}
	if err := yaml.Unmarshal(y, &body); err != nil {
		return nil, err
	}
	return json.Marshal(convert(body))
}
