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

package view

import (
	"html/template"
	"strings"

	"knative.dev/pkg/test/webhook-apicoverage/coveragecalculator"
)

type HtmlDisplayData struct {
	TypeCoverages   []coveragecalculator.TypeCoverage
	CoverageNumbers *coveragecalculator.CoverageValues
}

// GetHTMLDisplay is a helper method to display API Coverage details in
// json-like format inside a HTML page.
func GetHTMLDisplay(coverageData []coveragecalculator.TypeCoverage,
	coverageValues *coveragecalculator.CoverageValues) (string, error) {
	htmlData := HtmlDisplayData{
		TypeCoverages:   coverageData,
		CoverageNumbers: coverageValues,
	}

	tmpl, err := template.New("TypeCoverage").Parse(TypeCoverageTempl)
	if err != nil {
		return "", err
	}

	var buffer strings.Builder
	err = tmpl.Execute(&buffer, htmlData)
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}

// GetHTMLCoverageValuesDisplay is a helper method to display coverage values inside a HTML table.
func GetHTMLCoverageValuesDisplay(coverageValues *coveragecalculator.CoverageValues) (string, error) {
	tmpl, err := template.New("AggregateCoverage").Parse(AggregateCoverageTmpl)
	if err != nil {
		return "", err
	}

	var buffer strings.Builder
	err = tmpl.Execute(&buffer, coverageValues)
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}
