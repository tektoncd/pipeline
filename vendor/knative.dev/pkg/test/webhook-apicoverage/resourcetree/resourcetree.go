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
	"container/list"
	"reflect"

	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/test/webhook-apicoverage/coveragecalculator"
)

// ResourceTree encapsulates a tree corresponding to a resource type.
type ResourceTree struct {
	ResourceName string
	Root         NodeInterface
	Forest       *ResourceForest
}

// coverageDataHelper is a encapsulator parameter type to the BuildCoverageData method
// so as to avoid long parameter list.
type coverageDataHelper struct {
	typeCoverage  *[]coveragecalculator.TypeCoverage
	nodeRules     NodeRules
	fieldRules    FieldRules
	ignoredFields coveragecalculator.IgnoredFields
	coveredTypes  sets.String
}

func (r *ResourceTree) createNode(field string, parent NodeInterface, t reflect.Type) NodeInterface {
	var n NodeInterface
	switch t.Kind() {
	case reflect.Struct:
		n = new(StructKindNode)
	case reflect.Array, reflect.Slice:
		n = new(ArrayKindNode)
	case reflect.Ptr, reflect.UnsafePointer, reflect.Uintptr:
		n = new(PtrKindNode)
	case reflect.Bool, reflect.String, reflect.Float32, reflect.Float64,
		reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n = new(BasicTypeKindNode)
	default:
		n = new(OtherKindNode) // Maps, interfaces, etc
	}

	n.initialize(field, parent, t, r)

	if len(t.PkgPath()) != 0 {
		typeName := t.PkgPath() + "." + t.Name()
		if _, ok := r.Forest.ConnectedNodes[typeName]; !ok {
			r.Forest.ConnectedNodes[typeName] = list.New()
		}
		r.Forest.ConnectedNodes[typeName].PushBack(n)
	}

	return n
}

// BuildResourceTree builds a resource tree by calling into analyzeType method starting from root.
func (r *ResourceTree) BuildResourceTree(t reflect.Type) {
	r.Root = r.createNode(r.ResourceName, nil, t)
	r.Root.buildChildNodes(t)
}

// UpdateCoverage updates coverage data in the resource tree based on the provided reflect.Value
func (r *ResourceTree) UpdateCoverage(v reflect.Value) {
	r.Root.updateCoverage(v)
}

// BuildCoverageData calculates the coverage information for a resource tree by applying provided Node and Field rules.
func (r *ResourceTree) BuildCoverageData(nodeRules NodeRules, fieldRules FieldRules,
	ignoredFields coveragecalculator.IgnoredFields) []coveragecalculator.TypeCoverage {
	coverageHelper := coverageDataHelper{
		nodeRules:     nodeRules,
		fieldRules:    fieldRules,
		typeCoverage:  &[]coveragecalculator.TypeCoverage{},
		ignoredFields: ignoredFields,
		coveredTypes:  sets.String{},
	}
	r.Root.buildCoverageData(coverageHelper)
	return *coverageHelper.typeCoverage
}
