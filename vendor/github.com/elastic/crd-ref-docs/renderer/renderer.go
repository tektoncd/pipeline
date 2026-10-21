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
	"path/filepath"
	"text/template"

	"github.com/elastic/crd-ref-docs/config"
	"github.com/elastic/crd-ref-docs/types"
)

const mainTemplate = "gvList"

type Renderer interface {
	Render(gvd []types.GroupVersionDetails) error
}

func New(conf *config.Config) (Renderer, error) {
	switch conf.Renderer {
	case "asciidoctor":
		return NewAsciidoctorRenderer(conf)
	case "markdown":
		return NewMarkdownRenderer(conf)
	default:
		return nil, fmt.Errorf("unknown renderer: %s", conf.Renderer)
	}
}

func loadTemplate(templatesFS fs.FS, funcs template.FuncMap) (*template.Template, error) {
	return template.New("").Funcs(funcs).ParseFS(templatesFS, "*.tpl")
}

type funcMap struct {
	prefix string
	funcs  template.FuncMap
}

func combinedFuncMap(funcs ...funcMap) template.FuncMap {
	m := make(template.FuncMap)
	for _, f := range funcs {
		for k, v := range f.funcs {
			m[f.prefix+k] = v
		}
	}

	return m
}

// renderTemplate applies a given template to a set of GroupVersionDetails and writes the output to files, it supports
// two output modes as specified in the configuration: single mode or group mode.
// In single mode, all data is rendered into one output file.
// In group mode, separate files are created for each group.
func renderTemplate(tmpl *template.Template, conf *config.Config, fileExtension string, gvds []types.GroupVersionDetails) error {
	switch conf.OutputMode {
	case config.OutputModeSingle:
		fileName := fmt.Sprintf("%s.%s", "out", fileExtension)
		file, err := createOutFile(conf.OutputPath, false, fileName)
		defer file.Close()
		if err != nil {
			return err
		}

		if err := tmpl.ExecuteTemplate(file, mainTemplate, gvds); err != nil {
			return err
		}

	case config.OutputModeGroup:
		for _, gvd := range gvds {
			fileName := fmt.Sprintf("%s.%s", gvd.Group, fileExtension)
			file, err := createOutFile(conf.OutputPath, true, fileName)
			defer file.Close()
			if err != nil {
				return err
			}

			if err := tmpl.ExecuteTemplate(file, mainTemplate, []types.GroupVersionDetails{gvd}); err != nil {
				return err
			}
		}
	}

	return nil
}

// createOutFile creates the file pointed to by outputPath if it does not exist, or if it exists and is a directory,
// then creates a file in the directory using the given defaultFilename. If expectedDir is true, outputPath must be an
// existing directory where defaultFileName is created.
func createOutFile(outputPath string, expectedDir bool, defaultFileName string) (*os.File, error) {
	finfo, err := os.Stat(outputPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if finfo != nil && finfo.IsDir() {
		outputPath = filepath.Join(outputPath, defaultFileName)
	} else if expectedDir {
		return nil, fmt.Errorf("output path must point to an existing directory")
	}

	return os.Create(outputPath)
}
