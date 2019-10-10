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

package coveragecalculator

// CoverageValues encapsulates all the coverage related values.
type CoverageValues struct {
	TotalFields   int
	CoveredFields int
	IgnoredFields int

	PercentCoverage float64
}

func (c *CoverageValues) CalculatePercentageValue() {
	if c.TotalFields > 0 {
		c.PercentCoverage = (float64(c.CoveredFields) / float64(c.TotalFields-c.IgnoredFields)) * 100
	}
}

// CalculateTypeCoverage calculates aggregate coverage values based on provided []TypeCoverage
func CalculateTypeCoverage(typeCoverage []TypeCoverage) *CoverageValues {
	cv := CoverageValues{}
	for _, coverage := range typeCoverage {
		for _, field := range coverage.Fields {
			cv.TotalFields++
			if field.Ignored {
				cv.IgnoredFields++
			} else if field.Coverage {
				cv.CoveredFields++
			}
		}
	}
	return &cv
}
