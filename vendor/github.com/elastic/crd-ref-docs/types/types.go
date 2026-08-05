// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
package types

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

// Kind describes the kind of the type (alias, array, etc.)
type Kind int

const (
	AliasKind Kind = iota
	BasicKind
	InterfaceKind
	MapKind
	PointerKind
	SliceKind
	StructKind
	UnknownKind
	UnsupportedKind
)

func (k *Kind) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	switch strings.ToUpper(s) {
	case "ALIAS":
		*k = AliasKind
	case "BASIC":
		*k = BasicKind
	case "INTERFACE":
		*k = InterfaceKind
	case "MAP":
		*k = MapKind
	case "POINTER":
		*k = PointerKind
	case "SLICE":
		*k = SliceKind
	case "STRUCT":
		*k = StructKind
	case "UNKNOWN":
		*k = UnknownKind
	default:
		return fmt.Errorf("unknown kind %s", s)
	}
	return nil
}

func (k Kind) MarshalJSON() ([]byte, error) {
	kindStr := "UNKNOWN"
	switch k {
	case AliasKind:
		kindStr = "ALIAS"
	case BasicKind:
		kindStr = "BASIC"
	case InterfaceKind:
		kindStr = "INTERFACE"
	case MapKind:
		kindStr = "MAP"
	case PointerKind:
		kindStr = "POINTER"
	case SliceKind:
		kindStr = "SLICE"
	case StructKind:
		kindStr = "STRUCT"
	}

	return json.Marshal(kindStr)
}

// Type describes a declared type
type Type struct {
	UID            string                   `json:"uid"`
	Name           string                   `json:"name"`
	Package        string                   `json:"package"`
	Doc            string                   `json:"doc"`
	Default        string                   `json:"default"`
	Validation     []string                 `json:"validation"`
	Markers        markers.MarkerValues     `json:"markers"`
	GVK            *schema.GroupVersionKind `json:"gvk"`
	Kind           Kind                     `json:"kind"`
	Imported       bool                     `json:"imported"`
	UnderlyingType *Type                    `json:"underlyingType"` // for aliases, slices and pointers
	KeyType        *Type                    `json:"keyType"`        // for maps
	ValueType      *Type                    `json:"valueType"`      // for maps
	Fields         Fields                   `json:"fields"`         // for structs
	References     []*Type                  `json:"-"`              // other types that refer to this type
	EnumValues     []EnumValue              `json:"enumValues"`     // for enum values of aliased string types
}

func (t *Type) IsBasic() bool {
	switch t.Kind {
	case BasicKind:
		return true
	case SliceKind, PointerKind:
		return t.UnderlyingType != nil && t.UnderlyingType.IsBasic()
	case MapKind:
		return t.KeyType != nil && t.KeyType.IsBasic() && t.ValueType != nil && t.ValueType.IsBasic()
	case InterfaceKind:
		return true
	default:
		return false
	}
}

func (t *Type) Members() Fields {
	if t == nil {
		return nil
	}

	if len(t.Fields) > 0 {
		return t.Fields
	}

	switch t.Kind {
	case AliasKind, SliceKind, PointerKind:
		return t.UnderlyingType.Members()
	default:
		return nil
	}
}

func (t *Type) String() string {
	if t == nil {
		return "<unknown>"
	}

	var sb strings.Builder
	switch t.Kind {
	case MapKind:
		sb.WriteString("map[")
		sb.WriteString(t.KeyType.String())
		sb.WriteString("]")
		sb.WriteString(t.ValueType.String())
		return sb.String()
	case SliceKind:
		sb.WriteString("[]")
	case PointerKind:
		sb.WriteString("*")
	}
	if t.Package != "" {
		sb.WriteString(t.Package)
		sb.WriteString(".")
	}
	sb.WriteString(t.Name)

	return sb.String()
}

func (t *Type) IsAlias() bool {
	return t.Kind == AliasKind
}

func (t *Type) SortedReferences() []*Type {
	if t == nil || len(t.References) == 0 {
		return nil
	}

	sort.Slice(t.References, func(i, j int) bool {
		if t.References[i].Name < t.References[j].Name {
			return true
		}

		if t.References[i].Name == t.References[j].Name {
			return t.References[i].Package < t.References[j].Package
		}

		return false
	})

	return t.References
}

func (t *Type) ContainsInlinedTypes() bool {
	for _, f := range t.Members() {
		if f.Inlined {
			return true
		}
	}
	return false
}

// TypeMap is a map of Type elements
type TypeMap map[string]*Type

// PropagateMarkers propagates markers to struct fields and certain types, from
// their underlying types.
func (types TypeMap) PropagateMarkers() {
	for _, t := range types {
		t.propagateMarkers()
		for _, f := range t.Fields {
			f.propagateMarkers()
		}
	}
}

func (types TypeMap) InlineTypes(propagateReference func(original *Type, additional *Type)) {
	// If C is inlined in B, and B is inlined in A; the fields of C are copied
	// into B before the fields of B is copied into A. The ideal order of
	// iterating and inlining fields is NOT known. Worst-case, only one type's
	// fields are inlined in its parent type in each iteration.
	maxDepth := 100
	var numTypesToBeInlined int
	for iteration := 0; iteration < maxDepth; iteration++ {
		numTypesToBeInlined = 0
		for _, t := range types {
			// By iterating backwards, it is safe to delete field at current index
			// and copy the fields of the inlined type.
			for i := len(t.Fields) - 1; i >= 0; i-- {
				if !t.Fields[i].Inlined {
					continue
				}
				numTypesToBeInlined += 1

				embeddedType, ok := types[t.Fields[i].Type.UID]
				if !ok {
					zap.S().Warnw("Unable to find embedded type", "type", t,
						"embeddedType", t.Fields[i].Type)
					continue
				}

				// Only inline type's fields if the inlined type itself has no
				// types yet to be inlined.
				if !embeddedType.ContainsInlinedTypes() {
					zap.S().Debugw("Inlining embedded type", "type", t,
						"embeddedType", t.Fields[i].Type)
					t.Fields.inlineType(i, embeddedType)
					propagateReference(embeddedType, t)
				}
			}
		}
		if numTypesToBeInlined == 0 {
			return
		}
	}
	zap.S().Warnw("Failed to inline all inlined types", "remaining", numTypesToBeInlined)
}

func (t *Type) propagateMarkers() {
	if t.UnderlyingType == nil {
		return
	}

	switch t.Kind {
	case AliasKind, PointerKind:
		t.UnderlyingType.propagateMarkers()

		for name, marker := range t.UnderlyingType.Markers {
			if _, ok := t.Markers[name]; !ok {
				if t.Markers == nil {
					t.Markers = make(markers.MarkerValues)
				}
				t.Markers[name] = marker
			}
		}
	}
}

// Field describes a field in a struct.
type Field struct {
	Name       string
	Embedded   bool // Embedded struct in Go typing
	Inlined    bool // Inlined struct in serialization
	Doc        string
	Default    string
	Validation []string
	Markers    markers.MarkerValues
	Type       *Type
}

type Fields []*Field

func (f *Field) propagateMarkers() {
	f.Type.propagateMarkers()

	for name, marker := range f.Type.Markers {
		if _, ok := f.Markers[name]; !ok {
			if f.Markers == nil {
				f.Markers = make(markers.MarkerValues)
			}
			f.Markers[name] = marker
		}
	}
}

// inlineType replaces field at index i with the fields of inlined type.
func (fields *Fields) inlineType(i int, inlined *Type) {
	new := make([]*Field, 0, len(*fields)+len(inlined.Fields)-1)
	new = append(new, (*fields)[:i]...)
	new = append(new, inlined.Fields...)
	*fields = append(new, (*fields)[i+1:]...)
}

// Identifier generates the non-unique identifier for the type, disregarding it
// being a e.g. a list or pointer, and will thus be the same for both *Foo and
// Foo. Use Type's UID for a unique identifier.
func Identifier(t *Type) string {
	if t.Package == "" {
		return t.Name
	}

	return fmt.Sprintf("%s.%s", t.Package, t.Name)
}

// GroupVersionDetails encapsulates details about a discovered API group.
type GroupVersionDetails struct {
	schema.GroupVersion
	Doc     string
	Kinds   []string
	Types   TypeMap
	Markers markers.MarkerValues
}

func (gvd GroupVersionDetails) GroupVersionString() string {
	return gvd.GroupVersion.String()
}

func (gvd GroupVersionDetails) TypeForKind(k string) *Type {
	return gvd.Types[k]
}

func (gvd GroupVersionDetails) SortedTypes() []*Type {
	typeList := make([]*Type, len(gvd.Types))
	i := 0

	for _, t := range gvd.Types {
		typeList[i] = t
		i++
	}

	sort.Slice(typeList, func(i, j int) bool {
		return typeList[i].Name < typeList[j].Name
	})

	return typeList
}

func (gvd GroupVersionDetails) SortedKinds() []string {
	if len(gvd.Kinds) <= 1 {
		return gvd.Kinds
	}

	kindsList := make([]string, len(gvd.Kinds))
	copy(kindsList, gvd.Kinds)
	sort.Strings(kindsList)

	return kindsList
}

// EnumValue describes a constant value for enumerations
type EnumValue struct {
	Name string
	Doc  string
}
