/*
Copyright 2019 The Knative Authors

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

package coveragecalculator

import (
	"fmt"
	"io/ioutil"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	yaml "gopkg.in/yaml.v2"
)

// IgnoredFields encapsulates fields to be ignored in a package for API coverage calculation.
type IgnoredFields struct {
	ignoredFields map[string]sets.String
}

// This type is used for deserialization from the input .yaml file
type inputIgnoredFields struct {
	Package string   `yaml:"package"`
	Type    string   `yaml:"type"`
	Fields  []string `yaml:"fields"`
}

// ReadFromFile is a utility method that can be used by repos to read .yaml input file into
// IgnoredFields type.
func (ig *IgnoredFields) ReadFromFile(filePath string) error {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("Error reading file: %s Error : %v", filePath, err)
	}

	var inputEntries []inputIgnoredFields
	err = yaml.Unmarshal(data, &inputEntries)
	if err != nil {
		return fmt.Errorf("Error unmarshalling ignoredfields input yaml file: %s Content: %s Error: %v", filePath, string(data), err)
	}

	ig.ignoredFields = make(map[string]sets.String)

	for _, entry := range inputEntries {
		if _, ok := ig.ignoredFields[entry.Package]; !ok {
			ig.ignoredFields[entry.Package] = sets.String{}
		}

		for _, field := range entry.Fields {
			ig.ignoredFields[entry.Package].Insert(entry.Type + "." + field)
		}
	}
	return nil
}

// FieldIgnored method given a package, type and field returns true if the field is marked ignored.
func (ig *IgnoredFields) FieldIgnored(packageName string, typeName string, fieldName string) bool {
	if ig.ignoredFields != nil {
		for key, value := range ig.ignoredFields {
			if strings.HasSuffix(packageName, key) {
				if value.Has(typeName + "." + fieldName) {
					return true
				}
			}
		}
	}
	return false
}
