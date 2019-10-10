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

// rule.go contains different rules that can be defined to control resource tree traversal.

// NodeRules encapsulates all the node level rules defined by a repo.
type NodeRules struct {
	Rules []func(nodeInterface NodeInterface) bool
}

// Apply runs all the rules defined by a repo against a node.
func (n *NodeRules) Apply(node NodeInterface) bool {
	for _, rule := range n.Rules {
		if !rule(node) {
			return false
		}
	}
	return true
}

// FieldRules encapsulates all the field level rules defined by a repo.
type FieldRules struct {
	Rules []func(fieldName string) bool
}

// Apply runs all the rules defined by a repo against a field.
func (f *FieldRules) Apply(fieldName string) bool {
	for _, rule := range f.Rules {
		if !rule(fieldName) {
			return false
		}
	}
	return true
}
