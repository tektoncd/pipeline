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

package processor

import (
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"
	"regexp"
	"sort"
	"strings"

	"github.com/elastic/crd-ref-docs/config"
	"github.com/elastic/crd-ref-docs/types"
	"go.uber.org/zap"
	"golang.org/x/tools/go/packages"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-tools/pkg/crd"
	crdmarkers "sigs.k8s.io/controller-tools/pkg/crd/markers"
	"sigs.k8s.io/controller-tools/pkg/loader"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

const (
	objectRootMarker = "kubebuilder:object:root"
)

var ignoredCommentRegex = regexp.MustCompile(`\s*^(?i:\+|copyright)`)

type groupVersionInfo struct {
	schema.GroupVersion
	*loader.Package
	doc     string
	kinds   map[string]struct{}
	types   types.TypeMap
	markers markers.MarkerValues
}

func Process(config *config.Config) ([]types.GroupVersionDetails, error) {
	compiledConfig, err := compileConfig(config)
	if err != nil {
		return nil, err
	}

	p, err := newProcessor(compiledConfig, config.Flags.MaxDepth)
	if err != nil {
		return nil, err
	}
	// locate the packages annotated with group names
	if err := p.findAPITypes(config.SourcePath); err != nil {
		return nil, fmt.Errorf("failed to find API types in directory %s:%w", config.SourcePath, err)
	}

	p.types.InlineTypes(p.propagateReference)
	p.types.PropagateMarkers()
	p.parseMarkers()

	// collect references between types
	for typeName, refs := range p.references {
		typeDef, ok := p.types[typeName]
		if !ok {
			return nil, fmt.Errorf("type not loaded: %s", typeName)
		}

		for ref, _ := range refs {
			if rd, ok := p.types[ref]; ok {
				typeDef.References = append(typeDef.References, rd)
			}
		}
	}

	// build the return array
	var gvDetails []types.GroupVersionDetails
	for _, gvi := range p.groupVersions {
		details := types.GroupVersionDetails{GroupVersion: gvi.GroupVersion, Doc: gvi.doc}
		for k, _ := range gvi.kinds {
			details.Kinds = append(details.Kinds, k)
		}

		details.Types = make(types.TypeMap)
		for name, t := range gvi.types {
			key := types.Identifier(t)

			if p.shouldIgnoreType(key) {
				zap.S().Debugw("Skipping excluded type", "type", name)
				continue
			}
			if typeDef, ok := p.types[key]; ok && typeDef != nil {
				details.Types[name] = typeDef
			} else {
				zap.S().Fatalw("Type not loaded", "type", key)
			}
		}
		details.Markers = gvi.markers
		gvDetails = append(gvDetails, details)
	}

	// sort the array by GV
	sort.SliceStable(gvDetails, func(i, j int) bool {
		if gvDetails[i].Group < gvDetails[j].Group {
			return true
		}

		if gvDetails[i].Group == gvDetails[j].Group {
			return gvDetails[i].Version < gvDetails[j].Version
		}

		return false
	})

	return gvDetails, nil
}

func newProcessor(compiledConfig *compiledConfig, maxDepth int) (*processor, error) {
	registry, err := mkRegistry(compiledConfig.markers)
	if err != nil {
		return nil, err
	}
	p := &processor{
		compiledConfig: compiledConfig,
		maxDepth:       maxDepth,
		parser: &crd.Parser{
			Collector: &markers.Collector{Registry: registry},
			Checker:   &loader.TypeChecker{},
		},
		groupVersions: make(map[schema.GroupVersion]*groupVersionInfo),
		types:         make(types.TypeMap),
		references:    make(map[string]map[string]struct{}),
	}

	crd.AddKnownTypes(p.parser)
	return p, err
}

type processor struct {
	*compiledConfig
	maxDepth      int
	parser        *crd.Parser
	groupVersions map[schema.GroupVersion]*groupVersionInfo
	types         types.TypeMap
	references    map[string]map[string]struct{}
}

func (p *processor) findAPITypes(directory string) error {
	cfg := &packages.Config{Dir: directory}
	pkgs, err := loader.LoadRootsWithConfig(cfg, "./...")
	if err != nil {
		return err
	}

	for _, pkg := range pkgs {
		gvInfo := p.extractGroupVersionIfExists(p.parser.Collector, pkg)
		if gvInfo == nil {
			continue
		}

		if p.shouldIgnoreGroupVersion(gvInfo.GroupVersion.String()) {
			continue
		}

		// let the parser know that we need this package
		p.parser.AddPackage(pkg)

		// if we have encountered this GV before, use that instead
		if gv, ok := p.groupVersions[gvInfo.GroupVersion]; ok {
			gvInfo = gv
		} else {
			p.groupVersions[gvInfo.GroupVersion] = gvInfo
		}

		if gvInfo.types == nil {
			gvInfo.types = make(types.TypeMap)
		}

		// locate the kinds
		markers.EachType(p.parser.Collector, pkg, func(info *markers.TypeInfo) {
			// ignore types explicitly listed by the user
			if p.shouldIgnoreType(fmt.Sprintf("%s.%s", pkg.PkgPath, info.Name)) {
				return
			}

			// ignore unexported types
			if info.RawSpec.Name == nil || !info.RawSpec.Name.IsExported() {
				return
			}

			// load the type
			key := fmt.Sprintf("%s.%s", pkg.PkgPath, info.Name)
			typeDef, ok := p.types[key]
			if !ok {
				typeDef = p.processType(pkg, nil, pkg.TypesInfo.TypeOf(info.RawSpec.Name), 0)
			}

			if typeDef != nil && typeDef.Kind != types.BasicKind {
				gvInfo.types[info.Name] = typeDef
			}

			// is this a root object?
			if root := info.Markers.Get(objectRootMarker); root != nil {
				if gvInfo.kinds == nil {
					gvInfo.kinds = make(map[string]struct{})
				}
				gvInfo.kinds[info.Name] = struct{}{}
				typeDef.GVK = &schema.GroupVersionKind{Group: gvInfo.Group, Version: gvInfo.Version, Kind: info.Name}
			}

		})
	}

	return nil
}

func (p *processor) extractGroupVersionIfExists(collector *markers.Collector, pkg *loader.Package) *groupVersionInfo {
	markerValues, err := markers.PackageMarkers(collector, pkg)
	if err != nil {
		pkg.AddError(err)
		return nil
	}

	groupName := markerValues.Get("groupName")
	if groupName == nil {
		return nil
	}

	version := pkg.Name
	if v := markerValues.Get("versionName"); v != nil {
		version = v.(string)
	}

	gvInfo := &groupVersionInfo{
		GroupVersion: schema.GroupVersion{
			Group:   groupName.(string),
			Version: version,
		},
		Package: pkg,
		doc:     p.extractPkgDocumentation(pkg),
		markers: markerValues,
	}

	return gvInfo
}

func (p *processor) extractPkgDocumentation(pkg *loader.Package) string {
	var pkgComments []string

	pkg.NeedSyntax()
	for _, n := range pkg.Syntax {
		if n.Doc == nil {
			continue
		}
		comment := n.Doc.Text()
		commentLines := strings.Split(comment, "\n")
		for _, line := range commentLines {
			if !ignoredCommentRegex.MatchString(line) {
				pkgComments = append(pkgComments, line)
			}
		}
	}

	return strings.Join(pkgComments, "\n")
}

func (p *processor) processType(pkg *loader.Package, parentType *types.Type, t gotypes.Type, depth int) *types.Type {
	typeDef, rawType := mkType(pkg, t)
	typeID := types.Identifier(typeDef)
	if !rawType && p.shouldIgnoreType(typeID) {
		zap.S().Debugw("Skipping excluded type", "type", typeID)
		return nil
	}

	if processed, ok := p.types[typeDef.UID]; ok {
		return processed
	}

	info := p.parser.LookupType(pkg, typeDef.Name)
	if info != nil {
		typeDef.Doc = info.Doc
		typeDef.Markers = info.Markers

		if p.useRawDocstring && info.RawDecl != nil {
			// use raw docstring to support multi-line and indent preservation
			typeDef.Doc = strings.TrimSuffix(info.RawDecl.Doc.Text(), "\n")
		}
	}

	if depth > p.maxDepth {
		zap.S().Warnw("Not loading type due to reaching max recursion depth", "type", t.String())
		typeDef.Kind = types.UnknownKind
		return typeDef
	}

	zap.S().Debugw("Load", "package", typeDef.Package, "name", typeDef.Name)

	switch t := t.(type) {
	case nil:
		typeDef.Kind = types.UnknownKind
		zap.S().Warnw("Failed to determine AST type", "package", pkg.PkgPath, "type", t.String())

	case *gotypes.Named:
		// Import the type's package if not within current package
		if typeDef.Package != pkg.PkgPath {
			imports := pkg.Imports()
			importPkg, ok := imports[typeDef.Package]
			if !ok {
				zap.S().Warnw("Imported type cannot be found", "name", typeDef.Name, "package", typeDef.Package)
				return typeDef
			}
			p.parser.NeedPackage(importPkg)
			pkg = importPkg
		}

		typeDef.Kind = types.AliasKind
		underlying := t.Underlying()
		info := p.parser.LookupType(pkg, typeDef.Name)
		if info != nil {
			underlying = pkg.TypesInfo.TypeOf(info.RawSpec.Type)
		}
		if underlying.String() == "string" {
			typeDef.EnumValues = lookupConstantValuesForAliasedType(pkg, typeDef.Name)
		}
		typeDef.UnderlyingType = p.processType(pkg, typeDef, underlying, depth+1)
		p.addReference(typeDef, typeDef.UnderlyingType)

	case *gotypes.Struct:
		if parentType != nil {
			// Rather than the parent being a Named type with a "raw" Struct as
			// UnderlyingType, convert the parent to a Struct type.
			parentType.Kind = types.StructKind
			if info := p.parser.LookupType(pkg, parentType.Name); info != nil {
				p.processStructFields(parentType, pkg, info, depth)
			}
			// Abort processing type and return nil as UnderlyingType of parent.
			return nil
		} else {
			zap.S().Warnw("Anonymous structs are not supported", "package", pkg.PkgPath, "type", t.String())
			typeDef.Name = ""
			typeDef.Package = ""
			typeDef.Kind = types.UnsupportedKind
		}

	case *gotypes.Pointer:
		typeDef.Kind = types.PointerKind
		typeDef.UnderlyingType = p.processType(pkg, typeDef, t.Elem(), depth+1)
		if typeDef.UnderlyingType != nil {
			typeDef.Package = typeDef.UnderlyingType.Package
		}

	case *gotypes.Slice:
		typeDef.Kind = types.SliceKind
		typeDef.UnderlyingType = p.processType(pkg, typeDef, t.Elem(), depth+1)
		if typeDef.UnderlyingType != nil {
			typeDef.Package = typeDef.UnderlyingType.Package
		}

	case *gotypes.Map:
		typeDef.Kind = types.MapKind
		typeDef.KeyType = p.processType(pkg, typeDef, t.Key(), depth+1)
		typeDef.ValueType = p.processType(pkg, typeDef, t.Elem(), depth+1)
		if typeDef.ValueType != nil {
			typeDef.Package = typeDef.ValueType.Package
		}

	case *gotypes.Basic:
		typeDef.Kind = types.BasicKind
		typeDef.Package = ""

	case *gotypes.Interface:
		typeDef.Kind = types.InterfaceKind

	default:
		typeDef.Kind = types.UnsupportedKind
	}

	p.types[typeDef.UID] = typeDef
	return typeDef
}

func (p *processor) processStructFields(parentType *types.Type, pkg *loader.Package, info *markers.TypeInfo, depth int) {
	logger := zap.S().With("package", pkg.PkgPath, "type", parentType.String())
	logger.Debugw("Processing struct fields")
	parentTypeKey := types.Identifier(parentType)

	for _, f := range info.Fields {
		fieldDef := &types.Field{
			Name:     f.Name,
			Markers:  f.Markers,
			Doc:      f.Doc,
			Embedded: f.Name == "",
		}

		if tagVal, ok := f.Tag.Lookup("json"); ok {
			args := strings.Split(tagVal, ",")
			if len(args) > 0 && args[0] != "" {
				fieldDef.Name = args[0]
			}
			if len(args) > 1 && args[1] == "inline" {
				fieldDef.Inlined = true
			}
		}

		t := pkg.TypesInfo.TypeOf(f.RawField.Type)
		if t == nil {
			zap.S().Debugw("Failed to determine type of field", "field", fieldDef.Name)
			continue
		}

		logger.Debugw("Loading field type", "field", fieldDef.Name)
		if fieldDef.Type = p.processType(pkg, nil, t, depth); fieldDef.Type == nil {
			logger.Debugw("Failed to load type for field", "field", f.Name, "type", t.String())
			continue
		}

		// Keep old behaviour, where struct fields are never regarded as imported
		fieldDef.Type.Imported = false

		if fieldDef.Name == "" {
			fieldDef.Name = fieldDef.Type.Name
		}

		if p.shouldIgnoreField(parentTypeKey, fieldDef.Name) {
			zap.S().Debugw("Skipping excluded field", "type", parentType.String(), "field", fieldDef.Name)
			continue
		}

		parentType.Fields = append(parentType.Fields, fieldDef)
		p.addReference(parentType, fieldDef.Type)
	}
}

func mkType(pkg *loader.Package, t gotypes.Type) (*types.Type, bool) {
	qualifier := gotypes.RelativeTo(pkg.Types)
	cleanTypeName := strings.TrimLeft(gotypes.TypeString(t, qualifier), "*[]")

	typeDef := &types.Type{
		UID:     t.String(),
		Name:    cleanTypeName,
		Package: pkg.PkgPath,
	}

	// Check if the type is imported
	rawType := strings.HasPrefix(cleanTypeName, "struct{") || strings.HasPrefix(cleanTypeName, "interface{")
	dotPos := strings.LastIndexByte(cleanTypeName, '.')
	if !rawType && dotPos >= 0 {
		typeDef.Name = cleanTypeName[dotPos+1:]
		typeDef.Package = cleanTypeName[:dotPos]
		typeDef.Imported = true
	}

	return typeDef, rawType
}

// Every child that has a reference to 'originalType', will also get a reference to 'additionalType'.
func (p *processor) propagateReference(originalType *types.Type, additionalType *types.Type) {
	for _, parentRefs := range p.references {
		if _, ok := parentRefs[originalType.UID]; ok {
			parentRefs[additionalType.UID] = struct{}{}
		}
	}
}

func (p *processor) addReference(parent *types.Type, child *types.Type) {
	if child == nil {
		return
	}

	switch child.Kind {
	case types.SliceKind, types.PointerKind:
		p.addReference(parent, child.UnderlyingType)
	case types.MapKind:
		p.addReference(parent, child.KeyType)
		p.addReference(parent, child.ValueType)
	case types.AliasKind, types.StructKind:
		if p.references[child.UID] == nil {
			p.references[child.UID] = make(map[string]struct{})
		}
		p.references[child.UID][parent.UID] = struct{}{}
	}
}

func mkRegistry(customMarkers []config.Marker) (*markers.Registry, error) {
	registry := &markers.Registry{}
	if err := registry.Define(objectRootMarker, markers.DescribesType, true); err != nil {
		return nil, err
	}

	for _, marker := range crdmarkers.AllDefinitions {
		if err := registry.Register(marker.Definition); err != nil {
			return nil, err
		}
	}

	// Register k8s:* markers - sig apimachinery plans to unify CRD and native types on these.
	if err := registry.Define("k8s:required", markers.DescribesField, struct{}{}); err != nil {
		return nil, err
	}
	if err := registry.Define("k8s:optional", markers.DescribesField, struct{}{}); err != nil {
		return nil, err
	}

	for _, marker := range customMarkers {
		t := markers.DescribesField
		switch marker.Target {
		case config.TargetTypePackage:
			t = markers.DescribesPackage
		case config.TargetTypeType:
			t = markers.DescribesType
		case config.TargetTypeField:
			t = markers.DescribesField
		default:
			zap.S().Warnf("Skipping custom marker %s with unknown target type %s", marker.Name, marker.Target)
			continue
		}

		if err := registry.Define(marker.Name, t, struct{}{}); err != nil {
			return nil, fmt.Errorf("failed to define custom marker %s: %w", marker.Name, err)
		}
	}

	return registry, nil
}

func parseMarkers(markers markers.MarkerValues) (string, []string) {
	defaultValue := ""
	validation := []string{}

	markerNames := make([]string, 0, len(markers))
	for name := range markers {
		markerNames = append(markerNames, name)
	}
	sort.Strings(markerNames)

	for _, name := range markerNames {
		value := markers[name][len(markers[name])-1]

		if strings.HasPrefix(name, "kubebuilder:validation:") {
			name := strings.TrimPrefix(name, "kubebuilder:validation:")

			switch name {
			case "items:Pattern", "Pattern":
				value = fmt.Sprintf("`%s`", value)
			// FIXME: XValidation currently removed due to being long and difficult to read.
			// E.g. "XValidation: {self.page < 200 Please start a new book.}"
			case "XValidation":
				continue
			}
			validation = append(validation, fmt.Sprintf("%s: %v", name, value))
		}

		switch v := value.(type) {
		case crdmarkers.KubernetesDefault:
			defaultValue = fmt.Sprintf("%v", v.Value)
		case crdmarkers.Default:
			defaultValue = fmt.Sprintf("%v", v.Value)
		}

    // Handle standalone +required and +k8s:required marker
		// This is equivalent to +kubebuilder:validation:Required
		if name == "required" || name == "k8s:required" {
			validation = append(validation, "Required: {}")
		}
		// Handle standalone +optional and +k8s:optional marker
		// This is equivalent to +kubebuilder:validation:Optional
		if name == "optional" || name == "k8s:optional" {
			validation = append(validation, "Optional: {}")
		}
	}

	if strings.HasPrefix(defaultValue, "map[") {
		defaultValue = strings.TrimPrefix(defaultValue, "map[")
		defaultValue = strings.TrimSuffix(defaultValue, "]")
		defaultValue = fmt.Sprintf("{ %s }", defaultValue)
	}

	return defaultValue, validation
}

func (p *processor) parseMarkers() {
	for _, t := range p.types {
		t.Default, t.Validation = parseMarkers(t.Markers)
		for _, f := range t.Fields {
			f.Default, f.Validation = parseMarkers(f.Markers)
		}
	}
}

func lookupConstantValuesForAliasedType(pkg *loader.Package, aliasTypeName string) []types.EnumValue {
	values := []types.EnumValue{}
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			node, ok := decl.(*ast.GenDecl)
			if !ok || node.Tok != token.CONST {
				continue
			}
			for _, spec := range node.Specs {
				// look for constant declaration
				v, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				// value type must match the alias type name and have exactly one value
				if id, ok := v.Type.(*ast.Ident); !ok || id.String() != aliasTypeName || len(v.Values) != 1 {
					continue
				}
				// convert to a basic type to access to the value
				b, ok := v.Values[0].(*ast.BasicLit)
				if !ok {
					continue
				}
				values = append(values, types.EnumValue{
					// remove the '"' signs from the start and end of the value
					Name: b.Value[1 : len(b.Value)-1],
					Doc:  v.Doc.Text(),
				})
			}
		}
	}
	return values
}
