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

//test_util contains types defined and used by types and their corresponding verification methods.

import (
	"container/list"
	"errors"
	"fmt"
	"reflect"
)

const (
	basicTypeName    = "BasicType"
	ptrTypeName      = "PtrType"
	arrayTypeName    = "ArrayType"
	otherTypeName    = "OtherType"
	combinedTypeName = "CombinedType"
)

type baseType struct {
	field1 string
	field2 int16
}

type ptrType struct {
	structPtr *baseType
	basePtr   *float32
}

type arrayType struct {
	structArr []baseType
	baseArr   []bool
}

type otherType struct {
	structMap map[string]baseType
	baseMap   map[string]string
}

type combinedNodeType struct {
	b baseType
	a arrayType
	p ptrType
}

func getBaseTypeValue() baseType {
	return baseType{
		field1: "test",
	}
}

func getPtrTypeValueAllCovered() ptrType {
	f := new(float32)
	*f = 3.142
	b := getBaseTypeValue()
	return ptrType{
		basePtr:   f,
		structPtr: &b,
	}
}

func getPtrTypeValueSomeCovered() ptrType {
	b := getBaseTypeValue()
	return ptrType{
		structPtr: &b,
	}
}

func getArrValueAllCovered() arrayType {
	b1 := getBaseTypeValue()
	b2 := baseType{
		field2: 32,
	}

	return arrayType{
		structArr: []baseType{b1, b2},
		baseArr:   []bool{true, false},
	}
}

func getArrValueSomeCovered() arrayType {
	return arrayType{
		structArr: []baseType{getBaseTypeValue()},
	}
}

func getOtherTypeValue() otherType {
	m := make(map[string]baseType)
	m["test"] = getBaseTypeValue()
	return otherType{
		structMap: m,
	}
}

func getTestTree(treeName string, t reflect.Type) *ResourceTree {
	forest := ResourceForest{
		Version:        "TestVersion",
		ConnectedNodes: make(map[string]*list.List),
		TopLevelTrees:  make(map[string]ResourceTree),
	}

	tree := ResourceTree{
		ResourceName: treeName,
		Forest:       &forest,
	}

	tree.BuildResourceTree(t)
	forest.TopLevelTrees[treeName] = tree
	return &tree
}

func verifyBaseTypeNode(logPrefix string, data NodeData) error {
	if len(data.Children) != 2 {
		return fmt.Errorf("%s Expected 2 Children got only : %d", logPrefix, len(data.Children))
	}

	if value, ok := data.Children["field1"]; ok {
		n := value.GetData()
		if !n.LeafNode || n.FieldType.Kind() != reflect.String || n.FieldType.PkgPath() != "" || len(n.Children) != 0 {
			return fmt.Errorf("%s Unexpected field: field1. Expected LeafNode:true, Kind: %s, pkgPath: '' Children: 0 Found LeafNode: %t Kind: %s pkgPath: %s Children:%d",
				logPrefix, reflect.String, n.LeafNode, n.FieldType.Kind(), n.FieldType.PkgPath(), len(n.Children))
		}
	} else {
		return fmt.Errorf("%s field1 child Not found", logPrefix)
	}

	return nil
}

func verifyPtrNode(data NodeData) error {
	if len(data.Children) != 2 {
		return fmt.Errorf("Expected 2 Children got: %d", len(data.Children))
	}

	child := data.Children["structPtr"]
	if len(child.GetData().Children) != 1 {
		return fmt.Errorf("Unexpected size for field:structPtr. Expected : 1, Found : %d", len(child.GetData().Children))
	}

	child = child.GetData().Children["structPtr-ptr"]
	if err := verifyBaseTypeNode("child structPtr-ptr: ", child.GetData()); err != nil {
		return err
	}

	child = data.Children["basePtr"]
	if len(child.GetData().Children) != 1 {
		return fmt.Errorf("Unexpected size for field:basePtr. Expected : 1 Found : %d", len(child.GetData().Children))
	}

	child = child.GetData().Children["basePtr-ptr"]
	d := child.GetData()
	if d.FieldType.Kind() != reflect.Float32 || !d.LeafNode || d.FieldType.PkgPath() != "" || len(d.Children) != 0 {
		return fmt.Errorf("Unexpected field:basePtr-ptr: Expected: Kind: %s, LeafNode: true, pkgPath: '' Children: 0 Found Kind: %s, LeafNode: %t, pkgPath: %s Children:%d",
			reflect.Float32, d.FieldType.Kind(), d.LeafNode, d.FieldType.PkgPath(), len(d.Children))
	}

	return nil
}

func verifyArrayNode(data NodeData) error {
	if len(data.Children) != 2 {
		return fmt.Errorf("Expected 2 Children got: %d", len(data.Children))
	}

	child := data.Children["structArr"]
	d := child.GetData()
	if d.FieldType.Kind() != reflect.Slice {
		return fmt.Errorf("Unexpected kind for field:structArr: Expected : %s Found: %s", reflect.Slice, d.FieldType.Kind())
	} else if len(d.Children) != 1 {
		return fmt.Errorf("Unexpected number of Children for field:structArr: Expected : 1 Found : %d", len(d.Children))
	}

	child = child.GetData().Children["structArr-arr"]
	if err := verifyBaseTypeNode("child structArr-arr:", child.GetData()); err != nil {
		return err
	}

	child = data.Children["baseArr"]
	d = child.GetData()
	if d.FieldType.Kind() != reflect.Slice {
		return fmt.Errorf("Unexpected kind for field:baseArr: Expected : %s Found : %s", reflect.Slice, d.FieldType.Kind())
	} else if len(d.Children) != 1 {
		return fmt.Errorf("Unexpected number of Children for field:baseArr: Expected : 1 Found : %d", len(d.Children))
	}

	child = child.GetData().Children["baseArr-arr"]
	d = child.GetData()
	if d.FieldType.Kind() != reflect.Bool || !d.LeafNode || d.FieldType.PkgPath() != "" || len(d.Children) != 0 {
		return fmt.Errorf("Unexpected field:baseArr-arr Expected kind: %s, LeafNode: true, pkgPath: '', Children:0 Found: kind: %s, LeafNode: %t, pkgPath: %s, Children:%d",
			reflect.Bool, d.FieldType.Kind(), d.LeafNode, d.FieldType.PkgPath(), len(d.Children))
	}

	return nil
}

func verifyOtherTypeNode(data NodeData) error {
	if len(data.Children) != 2 {
		return fmt.Errorf("OtherTypeVerification: Expected 2 Children got: %d", len(data.Children))
	}

	child := data.Children["structMap"]
	d := child.GetData()
	if d.FieldType.Kind() != reflect.Map || !d.LeafNode || len(d.Children) != 0 {
		return fmt.Errorf("Unexpected field:structMap - Expected Kind: %s, LeafNode: true, Children:0 Found Kind: %s, LeafNode: %t, Children: %d",
			reflect.Map, d.FieldType.Kind(), d.LeafNode, len(d.Children))
	}

	child = data.Children["baseMap"]
	d = child.GetData()
	if d.FieldType.Kind() != reflect.Map || !d.LeafNode || len(d.Children) != 0 {
		return fmt.Errorf("Unexpected field:structMap - Expected Kind: %s, LeafNode: true, Children: 0 Found kind: %s, LeafNode: %t, Children: %d",
			reflect.Map, d.FieldType.Kind(), d.LeafNode, len(d.Children))
	}

	return nil
}

func verifyResourceForest(forest *ResourceForest) error {
	if len(forest.ConnectedNodes) != 4 {
		return fmt.Errorf("Invalid number of connected nodes found. Expected : 4, Found : %d", len(forest.ConnectedNodes))
	}

	baseType := reflect.TypeOf(baseType{})
	if value, found := forest.ConnectedNodes[baseType.PkgPath()+"."+baseType.Name()]; !found {
		return errors.New("Cannot find baseType{} connectedNode")
	} else if value.Len() != 3 {
		return fmt.Errorf("Invalid length of baseType{} Node. Expected : 3 Found : %d", value.Len())
	}

	arrayType := reflect.TypeOf(arrayType{})
	if value, found := forest.ConnectedNodes[arrayType.PkgPath()+"."+arrayType.Name()]; !found {
		return errors.New("Cannot find arrayType{} connectedNode")
	} else if value.Len() != 1 {
		return fmt.Errorf("Invalid length of arrayType{} Node. Expected : 1 Found : %d", value.Len())
	}

	return nil
}

func verifyBaseTypeValue(logPrefix string, node NodeInterface) error {
	if !node.GetData().Covered {
		return errors.New(logPrefix + " Node marked as not-Covered. Expected to be Covered")
	}

	if !node.GetData().Children["field1"].GetData().Covered {
		return errors.New(logPrefix + " field1 marked as not-Covered. Expected to be Covered")
	}

	if node.GetData().Children["field2"].GetData().Covered {
		return errors.New(logPrefix + " field2 marked as Covered. Expected to be not-Covered")
	}

	return nil
}

func verifyPtrValueAllCovered(node NodeInterface) error {
	if !node.GetData().Covered {
		return errors.New("Node marked as not-Covered. Expected to be Covered")
	}

	child := node.GetData().Children["basePtr"]
	if !child.GetData().Covered {
		return errors.New("field:base_ptr marked as not-Covered. Expected to be Covered")
	}

	if !child.GetData().Children["basePtr"+ptrNodeNameSuffix].GetData().Covered {
		return errors.New("field:basePtr" + ptrNodeNameSuffix + "marked as not-Covered. Expected to be Covered")
	}

	child = node.GetData().Children["structPtr"]
	if !child.GetData().Covered {
		return errors.New("field:structPtr marked as not-Covered. Expected to be Covered")
	}

	if err := verifyBaseTypeValue("field:structPtr"+ptrNodeNameSuffix, child.GetData().Children["structPtr"+ptrNodeNameSuffix]); err != nil {
		return err
	}

	return nil
}

func verifyPtrValueSomeCovered(node NodeInterface) error {
	if !node.GetData().Covered {
		return errors.New("Node marked as not-Covered. Expected to be Covered")
	}

	child := node.GetData().Children["basePtr"]
	if child.GetData().Covered {
		return errors.New("field:basePtr marked as Covered. Expected to be not-Covered")
	}

	if child.GetData().Children["basePtr"+ptrNodeNameSuffix].GetData().Covered {
		return errors.New("field:basePtr" + ptrNodeNameSuffix + "marked as Covered. Expected to be not-Covered")
	}

	child = node.GetData().Children["structPtr"]
	if !child.GetData().Covered {
		return errors.New("field:structPtr marked as not-Covered. Expected to be Covered")
	}

	if err := verifyBaseTypeValue("field:structPtr"+ptrNodeNameSuffix, child.GetData().Children["structPtr"+ptrNodeNameSuffix]); err != nil {
		return err
	}

	return nil
}

func verifyArryValueAllCovered(node NodeInterface) error {
	if !node.GetData().Covered {
		return errors.New("Node marked as not-Covered. Expected to be Covered")
	}

	child := node.GetData().Children["baseArr"]
	if !child.GetData().Covered {
		return errors.New("field:baseArr marked as not-Covered. Expected to be Covered")
	}

	if !child.GetData().Children["baseArr"+arrayNodeNameSuffix].GetData().Covered {
		return errors.New("field:baseArr" + arrayNodeNameSuffix + " marked as not-Covered. Expected to be Covered")
	}

	child = node.GetData().Children["structArr"]
	if !child.GetData().Covered {
		return errors.New("field:structArr marked as not-Covered. Expected to be Covered")
	}

	child = child.GetData().Children["structArr"+arrayNodeNameSuffix]
	if !child.GetData().Covered {
		return errors.New("structArr" + arrayNodeNameSuffix + " marked as not-Covered. Expected to be Covered")
	}

	if !child.GetData().Children["field1"].GetData().Covered {
		return errors.New("structArr" + arrayNodeNameSuffix + ".field1 marked as not-Covered. Expected to be Covered")
	}

	if !child.GetData().Children["field2"].GetData().Covered {
		return errors.New("structArr" + arrayNodeNameSuffix + ".field1 marked as not-Covered. Expected to be Covered")
	}

	return nil
}

func verifyArrValueSomeCovered(node NodeInterface) error {
	if !node.GetData().Covered {
		return errors.New("Node marked as not-Covered. Expected to be Covered")
	}

	child := node.GetData().Children["baseArr"]
	if child.GetData().Covered {
		return errors.New("field:baseArr marked as Covered. Expected to be not-Covered")
	}

	if child.GetData().Children["baseArr"+arrayNodeNameSuffix].GetData().Covered {
		return errors.New("field:baseArr" + arrayNodeNameSuffix + " marked as Covered. Expected to be not-Covered")
	}

	child = node.GetData().Children["structArr"]
	if !child.GetData().Covered {
		return errors.New("field:structArr marked as not-Covered. Expected to be Covered")
	}

	if err := verifyBaseTypeValue("field:structArr"+arrayNodeNameSuffix, child.GetData().Children["structArr"+arrayNodeNameSuffix]); err != nil {
		return err
	}

	return nil
}

func verifyOtherTypeValue(node NodeInterface) error {
	if !node.GetData().Covered {
		return errors.New("Node marked as not-Covered. Expected to be Covered")
	}

	if !node.GetData().Children["structMap"].GetData().Covered {
		return errors.New("field:structMap marked as not-Covered. Expected to be Covered")
	}

	if node.GetData().Children["baseMap"].GetData().Covered {
		return errors.New("field:baseMap marked as Covered. Expected to be not-Covered")
	}

	return nil
}
