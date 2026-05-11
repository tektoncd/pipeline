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

const (
	asciidocAnchorPrefix = "{anchor_prefix}-"
)

type AsciidoctorRenderer struct {
	conf *config.Config
	*Functions
}

func NewAsciidoctorRenderer(conf *config.Config) (*AsciidoctorRenderer, error) {
	baseFuncs, err := NewFunctions(conf)
	if err != nil {
		return nil, err
	}
	return &AsciidoctorRenderer{conf: conf, Functions: baseFuncs}, nil
}

func (adr *AsciidoctorRenderer) Render(gvd []types.GroupVersionDetails) error {
	funcMap := combinedFuncMap(funcMap{prefix: "asciidoc", funcs: adr.ToFuncMap()}, funcMap{funcs: sprig.TxtFuncMap()})

	var tpls fs.FS
	if adr.conf.TemplatesDir != "" {
		tpls = os.DirFS(adr.conf.TemplatesDir)
	} else {
		sub, err := fs.Sub(templates.Root, "asciidoctor")
		if err != nil {
			return err
		}
		tpls = sub
	}

	tmpl, err := loadTemplate(tpls, funcMap)
	if err != nil {
		return err
	}

	return renderTemplate(tmpl, adr.conf, "asciidoc", gvd)
}

func (adr *AsciidoctorRenderer) ToFuncMap() template.FuncMap {
	return template.FuncMap{
		"GroupVersionID":     adr.GroupVersionID,
		"RenderAnchorID":     adr.RenderAnchorID,
		"RenderExternalLink": adr.RenderExternalLink,
		"RenderGVLink":       adr.RenderGVLink,
		"RenderLocalLink":    adr.RenderLocalLink,
		"RenderType":         adr.RenderType,
		"RenderTypeLink":     adr.RenderTypeLink,
		"SafeID":             adr.SafeID,
		"ShouldRenderType":   adr.ShouldRenderType,
		"TypeID":             adr.TypeID,
		"RenderFieldDoc":     adr.RenderFieldDoc,
		"RenderValidation":   adr.RenderValidation,
		"TemplateValue":      adr.TemplateValue,
	}
}

func (adr *AsciidoctorRenderer) ShouldRenderType(t *types.Type) bool {
	return t != nil && (t.GVK != nil || len(t.References) > 0)
}

func (adr *AsciidoctorRenderer) RenderType(t *types.Type) string {
	var sb strings.Builder
	switch t.Kind {
	case types.MapKind:
		sb.WriteString("object (")
		sb.WriteString("keys:")
		sb.WriteString(adr.RenderTypeLink(t.KeyType))
		sb.WriteString(", values:")
		sb.WriteString(adr.RenderTypeLink(t.ValueType))
		sb.WriteString(")")
	case types.SliceKind:
		sb.WriteString(adr.RenderTypeLink(t.UnderlyingType))
		sb.WriteString(" array")
	default:
		sb.WriteString(adr.RenderTypeLink(t))
	}

	return sb.String()
}

func (adr *AsciidoctorRenderer) RenderTypeLink(t *types.Type) string {
	text := adr.SimplifiedTypeName(t)

	link, local := adr.LinkForType(t)
	if link == "" {
		return text
	}

	if local {
		return adr.RenderLocalLink(asciidocAnchorPrefix, link, text)
	} else {
		return adr.RenderExternalLink(link, text)
	}
}

func (adr *AsciidoctorRenderer) RenderLocalLink(prefix, link, text string) string {
	return fmt.Sprintf("xref:%s%s[$$%s$$]", prefix, link, text)
}

func (adr *AsciidoctorRenderer) RenderExternalLink(link, text string) string {
	return fmt.Sprintf("link:%s[$$%s$$]", link, text)
}

func (adr *AsciidoctorRenderer) RenderGVLink(gv types.GroupVersionDetails) string {
	return adr.RenderLocalLink(asciidocAnchorPrefix, adr.GroupVersionID(gv), gv.GroupVersionString())
}

func (adr *AsciidoctorRenderer) RenderAnchorID(id string) string {
	return fmt.Sprintf("%s%s", asciidocAnchorPrefix, adr.SafeID(id))
}

func (adr *AsciidoctorRenderer) TemplateValue(key string) string {
	if adr == nil || adr.conf == nil {
		return ""
	}
	return adr.conf.TemplateKeyValues.AsMap()[key]
}

func (adr *AsciidoctorRenderer) RenderFieldDoc(text string) string {
	// Escape the pipe character, which has special meaning for asciidoc as a way to format tables,
	// so that including | in a comment does not result in wonky tables.
	out := escapePipe(text)

	// Trim any leading and trailing whitespace from each line.
	lines := strings.Split(out, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
		// Replace newlines with hard line breaks so that newlines are rendered as expected for non-empty lines.
		// See: https://docs.asciidoctor.org/asciidoc/latest/blocks/hard-line-breaks
		if lines[i] != "" {
			lines[i] = lines[i] + " +"
		}
	}

	return strings.Join(lines, "\n")
}

func (adr *AsciidoctorRenderer) RenderValidation(text string) string {
	renderedText := escapeFirstAsterixInEachPair(text)
	renderedText = escapePipe(renderedText)
	return escapeCurlyBraces(renderedText)
}

// escapeFirstAsterixInEachPair escapes the first asterix in each pair of
// asterixes in text. E.g. "*a*b*c*" -> "\*a*b\*c*" and "*a*b*" -> "\*a*b*".
func escapeFirstAsterixInEachPair(text string) string {
	index := -1
	for i := 0; i < len(text); i++ {
		if text[i] == '*' {
			if index >= 0 {
				text = text[:index] + "\\" + text[index:]
				index = -1
				i++
			} else {
				index = i
			}
		}
	}
	return text
}

// escapePipe ensures sufficient escapes are added to pipe characters, so they are not mistaken
// for asciidoctor table formatting.
func escapePipe(text string) string {
	return strings.ReplaceAll(text, "|", "\\|")
}

// escapeCurlyBraces ensures sufficient escapes are added to curly braces, so they are not mistaken
// for asciidoctor id attributes.
func escapeCurlyBraces(text string) string {
	// Per asciidoctor docs, only the leading curly brace needs to be escaped.
	return strings.ReplaceAll(text, "{", "\\{")
}
