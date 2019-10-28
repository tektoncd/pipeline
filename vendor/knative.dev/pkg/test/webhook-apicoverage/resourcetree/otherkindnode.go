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

// OtherKindNode represents nodes in the resource tree of types like maps, interfaces, etc
type OtherKindNode struct {
	NodeData
}

// GetData returns node data
func (o *OtherKindNode) GetData() NodeData {
	return o.NodeData
}

func (o *OtherKindNode) initialize(field string, parent NodeInterface, t reflect.Type, rt *ResourceTree) {
	o.NodeData.initialize(field, parent, t, rt)
	o.NodeData.LeafNode = true
}

func (o *OtherKindNode) buildChildNodes(t reflect.Type) {}

func (o *OtherKindNode) updateCoverage(v reflect.Value) {
	if !v.IsNil() {
		o.Covered = true
	}
}

// no-op as the coverage is calculated as field coverage in parent node.
func (o *OtherKindNode) buildCoverageData(coverageHelper coverageDataHelper) {}

func (o *OtherKindNode) getValues() sets.String {
	return nil
}
