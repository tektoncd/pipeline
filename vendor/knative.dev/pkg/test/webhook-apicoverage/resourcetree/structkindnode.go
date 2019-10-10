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

package resourcetree

import (
	"reflect"

	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	v1TimeType       = "v1.Time"
	volatileTimeType = "apis.VolatileTime"
)

// StructKindNode represents nodes in the resource tree of type reflect.Kind.Struct
type StructKindNode struct {
	NodeData
}

// GetData returns node data
func (s *StructKindNode) GetData() NodeData {
	return s.NodeData
}

func (s *StructKindNode) initialize(field string, parent NodeInterface, t reflect.Type, rt *ResourceTree) {
	s.NodeData.initialize(field, parent, t, rt)
}

func (s *StructKindNode) buildChildNodes(t reflect.Type) {
	// For types that are part of the standard package, we treat them as leaf nodes and don't expand further.
	// https://golang.org/pkg/reflect/#StructField.
	if len(s.FieldType.PkgPath()) == 0 {
		s.LeafNode = true
		return
	}

	for i := 0; i < t.NumField(); i++ {
		var childNode NodeInterface
		if s.isTimeNode(t.Field(i).Type) {
			childNode = new(TimeTypeNode)
			childNode.initialize(t.Field(i).Name, s, t.Field(i).Type, s.Tree)
		} else {
			childNode = s.Tree.createNode(t.Field(i).Name, s, t.Field(i).Type)
		}
		s.Children[t.Field(i).Name] = childNode
		childNode.buildChildNodes(t.Field(i).Type)
	}
}

func (s *StructKindNode) isTimeNode(t reflect.Type) bool {
	if t.Kind() == reflect.Struct {
		return t.String() == v1TimeType || t.String() == volatileTimeType
	} else if t.Kind() == reflect.Ptr {
		return t.Elem().String() == v1TimeType || t.String() == volatileTimeType
	} else {
		return false
	}
}

func (s *StructKindNode) updateCoverage(v reflect.Value) {
	if v.IsValid() {
		s.Covered = true
		if !s.LeafNode {
			for i := 0; i < v.NumField(); i++ {
				s.Children[v.Type().Field(i).Name].updateCoverage(v.Field(i))
			}
		}
	}
}

func (s *StructKindNode) buildCoverageData(coverageHelper coverageDataHelper) {
	if len(s.Children) == 0 {
		return
	}

	coverage := s.Tree.Forest.getConnectedNodeCoverage(s.FieldType, coverageHelper.fieldRules, coverageHelper.ignoredFields)
	*coverageHelper.typeCoverage = append(*coverageHelper.typeCoverage, coverage)
	// Adding the type to covered fields so as to avoid revisiting the same node in other parts of the resource tree.
	coverageHelper.coveredTypes.Insert(s.FieldType.PkgPath() + "." + s.FieldType.Name())

	for field := range coverage.Fields {
		node := s.Children[field]
		if !coverage.Fields[field].Ignored && node.GetData().Covered && coverageHelper.nodeRules.Apply(node) {
			// Check to see if the type has already been covered.
			if !coverageHelper.coveredTypes.Has(node.GetData().FieldType.PkgPath() + "." + node.GetData().FieldType.Name()) {
				node.buildCoverageData(coverageHelper)
			}
		}
	}
}

func (s *StructKindNode) getValues() sets.String {
	return nil
}
