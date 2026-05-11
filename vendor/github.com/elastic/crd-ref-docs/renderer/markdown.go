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
	"fmt"
	"io/fs"
	"os"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/elastic/crd-ref-docs/config"
	"github.com/elastic/crd-ref-docs/templates"
	"github.com/elastic/crd-ref-docs/types"
)

type MarkdownRenderer struct {
	conf *config.Config
	*Functions
}

func NewMarkdownRenderer(conf *config.Config) (*MarkdownRenderer, error) {
	baseFuncs, err := NewFunctions(conf)
	if err != nil {
		return nil, err
	}
	return &MarkdownRenderer{conf: conf, Functions: baseFuncs}, nil
}

func (m *MarkdownRenderer) Render(gvd []types.GroupVersionDetails) error {
	funcMap := combinedFuncMap(funcMap{prefix: "markdown", funcs: m.ToFuncMap()}, funcMap{funcs: sprig.TxtFuncMap()})

	var tpls fs.FS
	if m.conf.TemplatesDir != "" {
		tpls = os.DirFS(m.conf.TemplatesDir)
	} else {
		sub, err := fs.Sub(templates.Root, "markdown")
		if err != nil {
			return err
		}
		tpls = sub
	}

	tmpl, err := loadTemplate(tpls, funcMap)
	if err != nil {
		return err
	}

	return renderTemplate(tmpl, m.conf, "md", gvd)
}

func (m *MarkdownRenderer) ToFuncMap() template.FuncMap {
	return template.FuncMap{
		"GroupVersionID":     m.GroupVersionID,
		"RenderExternalLink": m.RenderExternalLink,
		"RenderGVLink":       m.RenderGVLink,
		"RenderLocalLink":    m.RenderLocalLink,
		"RenderType":         m.RenderType,
		"RenderTypeLink":     m.RenderTypeLink,
		"SafeID":             m.SafeID,
		"ShouldRenderType":   m.ShouldRenderType,
		"TypeID":             m.TypeID,
		"RenderFieldDoc":     m.RenderFieldDoc,
		"RenderDefault":      m.RenderDefault,
		"TemplateValue":      m.TemplateValue,
	}
}

func (m *MarkdownRenderer) ShouldRenderType(t *types.Type) bool {
	return t != nil && (t.GVK != nil || len(t.References) > 0)
}

func (m *MarkdownRenderer) RenderType(t *types.Type) string {
	var sb strings.Builder
	switch t.Kind {
	case types.MapKind:
		sb.WriteString("object (")
		sb.WriteString("keys:")
		sb.WriteString(m.RenderTypeLink(t.KeyType))
		sb.WriteString(", values:")
		sb.WriteString(m.RenderTypeLink(t.ValueType))
		sb.WriteString(")")
	case types.SliceKind:
		sb.WriteString(m.RenderTypeLink(t.UnderlyingType))
		sb.WriteString(" array")
	default:
		sb.WriteString(m.RenderTypeLink(t))
	}

	return sb.String()
}

func (m *MarkdownRenderer) RenderTypeLink(t *types.Type) string {
	text := m.SimplifiedTypeName(t)

	link, local := m.LinkForType(t)
	if link == "" {
		return text
	}

	if local {
		return m.RenderLocalLink(text)
	} else {
		return m.RenderExternalLink(link, text)
	}
}

func (m *MarkdownRenderer) RenderLocalLink(text string) string {
	anchor := strings.ToLower(
		strings.NewReplacer(
			" ", "-",
			".", "",
			"/", "",
			"(", "",
			")", "",
		).Replace(text),
	)
	return fmt.Sprintf("[%s](#%s)", text, anchor)
}

func (m *MarkdownRenderer) TemplateValue(key string) string {
	if m == nil || m.conf == nil {
		return ""
	}
	return m.conf.TemplateKeyValues.AsMap()[key]
}

func (m *MarkdownRenderer) RenderExternalLink(link, text string) string {
	return fmt.Sprintf("[%s](%s)", text, link)
}

func (m *MarkdownRenderer) RenderGVLink(gv types.GroupVersionDetails) string {
	return m.RenderLocalLink(gv.GroupVersionString())
}

func (m *MarkdownRenderer) RenderFieldDoc(text string) string {
	// Escape the pipe character, which has special meaning for Markdown as a way to format tables
	// so that including | in a comment does not result in wonky tables.
	out := strings.ReplaceAll(text, "|", "\\|")

	// Escape the curly bracket character.
	out = strings.ReplaceAll(out, "{", "\\{")
	out = strings.ReplaceAll(out, "}", "\\}")

	// Replace newlines with 1 line break so that they don't break the Markdown table formatting.
	out = strings.ReplaceAll(out, "\n", "<br />")
	// and remove double newline generated for empty lines
	// empty line is still rendered in the table, without removing the duplicate
	// newline it would be rendered as two empty lines
	return strings.ReplaceAll(out, "<br /><br />", "<br />")
}

func (m *MarkdownRenderer) RenderDefault(text string) string {
	return strings.NewReplacer(
		"{", "\\{",
		"}", "\\}",
	).Replace(text)
}
