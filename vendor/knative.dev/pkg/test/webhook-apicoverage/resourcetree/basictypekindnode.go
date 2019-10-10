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

package resourcetree

import (
	"fmt"
	"reflect"
	"strconv"

	"k8s.io/apimachinery/pkg/util/sets"
)

// BasicTypeKindNode represents resource tree node of basic types like int, float, etc.
type BasicTypeKindNode struct {
	NodeData
	values       sets.String // Values seen for this node. Useful for enum types.
	possibleEnum bool        // Flag to indicate if this is a possible enum.
}

// GetData returns node data
func (b *BasicTypeKindNode) GetData() NodeData {
	return b.NodeData
}

func (b *BasicTypeKindNode) initialize(field string, parent NodeInterface, t reflect.Type, rt *ResourceTree) {
	b.NodeData.initialize(field, parent, t, rt)
	b.values = sets.String{}
	b.NodeData.LeafNode = true
}

func (b *BasicTypeKindNode) buildChildNodes(t reflect.Type) {
	// Treating bools as possible enums to support tighter coverage information.
	if t.Name() != t.Kind().String() || b.FieldType.Kind() == reflect.Bool {
		b.possibleEnum = true
	}
}

func (b *BasicTypeKindNode) updateCoverage(v reflect.Value) {
	if value := b.string(v); len(value) > 0 {
		if b.possibleEnum || b.FieldType.Kind() == reflect.Bool {
			b.values.Insert(value)
		}
		b.Covered = true
	}
}

// no-op as the coverage is calculated as field coverage in parent node.
func (b *BasicTypeKindNode) buildCoverageData(coverageHelper coverageDataHelper) {}

func (b *BasicTypeKindNode) string(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.Int() != 0 {
			return strconv.Itoa(int(v.Int()))
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v.Uint() != 0 {
			return strconv.FormatUint(v.Uint(), 10)
		}
	case reflect.Float32, reflect.Float64:
		if v.Float() != 0 {
			return fmt.Sprintf("%f", v.Float())
		}
	case reflect.String:
		if v.Len() != 0 {
			return v.String()
		}
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	}

	return ""
}

func (b *BasicTypeKindNode) getValues() sets.String {
	if b.possibleEnum {
		return b.values
	}

	return nil
}
