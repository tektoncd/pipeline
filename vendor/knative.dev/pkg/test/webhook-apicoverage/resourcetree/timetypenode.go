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

// TimeTypeNode is a node type that encapsulates fields that are internally time based. E.g metav1.ObjectMeta.CreationTimestamp or metav1.ObjectMeta.DeletionTimestamp.
// These are internally of type metav1.Time which use standard time type, but their values are specified as timestamp strings with parsing logic to create time objects. For
// use-case we only care if the value is set, so we create TimeTypeNodes and mark them as leafnodes.
type TimeTypeNode struct {
	NodeData
}

// GetData returns node data
func (ti *TimeTypeNode) GetData() NodeData {
	return ti.NodeData
}

func (ti *TimeTypeNode) initialize(field string, parent NodeInterface, t reflect.Type, rt *ResourceTree) {
	ti.NodeData.initialize(field, parent, t, rt)
	ti.LeafNode = true
}

func (ti *TimeTypeNode) buildChildNodes(t reflect.Type) {}

func (ti *TimeTypeNode) updateCoverage(v reflect.Value) {
	if v.Type().Kind() == reflect.Struct && v.IsValid() {
		ti.Covered = true
	} else if v.Type().Kind() == reflect.Ptr && !v.IsNil() {
		ti.Covered = true
	}
}

// no-op as the coverage is calculated as field coverage in parent node.
func (ti *TimeTypeNode) buildCoverageData(coverageHelper coverageDataHelper) {}

func (ti *TimeTypeNode) getValues() sets.String {
	return nil
}
