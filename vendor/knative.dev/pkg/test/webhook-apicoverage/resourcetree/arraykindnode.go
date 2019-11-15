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
	"reflect"

	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	arrayNodeNameSuffix = "-arr"
)

// ArrayKindNode represents resource tree node of types reflect.Kind.Array and reflect.Kind.Slice
type ArrayKindNode struct {
	NodeData
	// Array type e.g. []int will store reflect.Kind.Int.
	// This is required for type-expansion and value-evaluation decisions.
	arrKind reflect.Kind
}

// GetData returns node data
func (a *ArrayKindNode) GetData() NodeData {
	return a.NodeData
}

func (a *ArrayKindNode) initialize(field string, parent NodeInterface, t reflect.Type, rt *ResourceTree) {
	a.NodeData.initialize(field, parent, t, rt)
	a.arrKind = t.Elem().Kind()
}

func (a *ArrayKindNode) buildChildNodes(t reflect.Type) {
	childName := a.Field + arrayNodeNameSuffix
	childNode := a.Tree.createNode(childName, a, t.Elem())
	a.Children[childName] = childNode
	childNode.buildChildNodes(t.Elem())
}

func (a *ArrayKindNode) updateCoverage(v reflect.Value) {
	if !v.IsNil() {
		a.Covered = true
		for i := 0; i < v.Len(); i++ {
			a.Children[a.Field+arrayNodeNameSuffix].updateCoverage(v.Index(i))
		}
	}
}

func (a *ArrayKindNode) buildCoverageData(coverageHelper coverageDataHelper) {
	if a.arrKind == reflect.Struct {
		a.Children[a.Field+arrayNodeNameSuffix].buildCoverageData(coverageHelper)
	}
}

func (a *ArrayKindNode) getValues() sets.String {
	return nil
}
