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
	ptrNodeNameSuffix = "-ptr"
)

// PtrKindNode represents nodes in the resource tree of type reflect.Kind.Ptr, reflect.Kind.UnsafePointer, etc.
type PtrKindNode struct {
	NodeData
	objKind reflect.Kind // Type of the object being pointed to. Eg: *int will store reflect.Kind.Int. This is required for type-expansion and value-evaluation decisions.
}

// GetData returns node data
func (p *PtrKindNode) GetData() NodeData {
	return p.NodeData
}

func (p *PtrKindNode) initialize(field string, parent NodeInterface, t reflect.Type, rt *ResourceTree) {
	p.NodeData.initialize(field, parent, t, rt)
	p.objKind = t.Elem().Kind()
}

func (p *PtrKindNode) buildChildNodes(t reflect.Type) {
	childName := p.Field + ptrNodeNameSuffix
	childNode := p.Tree.createNode(childName, p, t.Elem())
	p.Children[childName] = childNode
	childNode.buildChildNodes(t.Elem())
}

func (p *PtrKindNode) updateCoverage(v reflect.Value) {
	if !v.IsNil() {
		p.Covered = true
		p.Children[p.Field+ptrNodeNameSuffix].updateCoverage(v.Elem())
	}
}

func (p *PtrKindNode) buildCoverageData(coverageHelper coverageDataHelper) {
	if p.objKind == reflect.Struct {
		p.Children[p.Field+ptrNodeNameSuffix].buildCoverageData(coverageHelper)
	}
}

func (p *PtrKindNode) getValues() sets.String {
	return nil
}
