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

package renderer

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/elastic/crd-ref-docs/config"
	"github.com/elastic/crd-ref-docs/types"
	"go.uber.org/zap"
)

const (
	kubePackagesRegex   = `^k8s\.io/(?:api|apimachinery|apiextensions-apiserver/pkg/apis)/`
	kubeDocLinkTemplate = `https://kubernetes.io/docs/reference/generated/kubernetes-api/v{{ .kubeVersion }}/#{{ .type }}-{{ .version }}-{{ .group }}`
)

type Functions struct {
	conf *config.Config
	*kubernetesHelper
	safeIDRegex *regexp.Regexp
}

func NewFunctions(conf *config.Config) (*Functions, error) {
	kubeHelper, err := newKubernetesHelper(conf)
	if err != nil {
		return nil, err
	}

	safeIDRegex, err := regexp.Compile("[[:punct:]]+")
	if err != nil {
		return nil, fmt.Errorf("failed to compile safe ID regex: %w", err)
	}

	return &Functions{
		conf:             conf,
		kubernetesHelper: kubeHelper,
		safeIDRegex:      safeIDRegex,
	}, nil
}

func (f *Functions) TypeID(t *types.Type) string {
	return f.SafeID(types.Identifier(t))
}

func (f *Functions) GroupVersionID(gv types.GroupVersionDetails) string {
	return f.SafeID(gv.GroupVersionString())
}

func (f *Functions) SafeID(id string) string {
	return strings.ToLower(f.safeIDRegex.ReplaceAllLiteralString(id, "-"))
}

func (f *Functions) LinkForType(t *types.Type) (link string, local bool) {
	if f.IsKubeType(t) {
		return f.LinkForKubeType(t), false
	}

	if kt, ok := f.IsKnownType(t); ok {
		return f.LinkForKnownType(kt), false
	}

	if t.IsBasic() || t.Imported {
		return "", false
	}

	return f.TypeID(t), true
}

func (f *Functions) SimplifiedTypeName(t *types.Type) string {
	if !t.IsBasic() {
		return t.Name
	}

	switch t.Kind {
	case types.BasicKind:
		return f.BasicTypeName(t.Name)
	case types.PointerKind:
		return f.BasicTypeName(t.UnderlyingType.Name)
	case types.SliceKind:
		return fmt.Sprintf("%s array", f.BasicTypeName(t.UnderlyingType.Name))
	case types.MapKind:
		return "object"
	default:
		return t.Name
	}
}

func (f *Functions) BasicTypeName(name string) string {
	switch name {
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "byte", "rune":
		return "integer"
	case "float32", "float64":
		return "float"
	case "bool":
		return "boolean"
	default:
		return name
	}
}

func (f *Functions) IsKnownType(t *types.Type) (*config.KnownType, bool) {
	for _, kt := range f.conf.Render.KnownTypes {
		if kt.Package == t.Package && t.Name == kt.Name {
			return kt, true
		}
	}

	return nil, false
}

func (f *Functions) LinkForKnownType(kt *config.KnownType) string {
	return kt.Link
}

type kubernetesHelper struct {
	kubeVersion     string
	packagesRegex   *regexp.Regexp
	docLinkTemplate *template.Template
}

func newKubernetesHelper(conf *config.Config) (*kubernetesHelper, error) {
	packagesRegex, err := regexp.Compile(kubePackagesRegex)
	if err != nil {
		return nil, fmt.Errorf("failed to compile kube package regex: %w", err)
	}

	docLinkTemplate, err := template.New("").Parse(kubeDocLinkTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kube doc link template: %w", err)
	}

	return &kubernetesHelper{
		kubeVersion:     conf.Render.KubernetesVersion,
		packagesRegex:   packagesRegex,
		docLinkTemplate: docLinkTemplate,
	}, nil
}

func (k *kubernetesHelper) IsKubeType(t *types.Type) bool {
	return k.packagesRegex.MatchString(t.Package)
}

func (k *kubernetesHelper) LinkForKubeType(t *types.Type) string {
	if !k.IsKubeType(t) {
		return ""
	}

	parts := strings.Split(t.Package, "/")
	if len(parts) < 2 {
		zap.S().Fatalw("Unexpected Kubernetes package name", "type", t)
	}
	group := strings.ToLower(parts[len(parts)-2])
	// this is alias handling
	if group == "apiextensions" {
		group = "apiextensions-k8s-io"
	}
	args := map[string]string{
		"kubeVersion": k.kubeVersion,
		"group":       group,
		"version":     strings.ToLower(parts[len(parts)-1]),
		"type":        strings.ToLower(t.Name),
	}

	s := new(bytes.Buffer)
	if err := k.docLinkTemplate.Execute(s, args); err != nil {
		zap.S().Fatalw("Failed to render Kube doc link", "type", t, "error", err)
	}

	return s.String()
}
