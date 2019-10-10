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
	"strings"
	"text/template"

	"knative.dev/pkg/test/webhook-apicoverage/coveragecalculator"
)

// GetCoveragePercentageXMLDisplay is a helper method to write resource coverage
// percentage values to junit xml file format.
func GetCoveragePercentageXMLDisplay(
	percentageCoverages *coveragecalculator.CoveragePercentages) (string, error) {
	tmpl, err := template.New("JunitResult").Parse(JunitResultTmpl)
	if err != nil {
		return "", err
	}

	var buffer strings.Builder
	err = tmpl.Execute(&buffer, percentageCoverages)
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}
