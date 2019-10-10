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

import (
	"math"
)

// CoverageValues encapsulates all the coverage related values.
type CoverageValues struct {
	TotalFields   int
	CoveredFields int
	IgnoredFields int

	PercentCoverage float64
}

// CoveragePercentages encapsulate percentage coverage for resources.
type CoveragePercentages struct {

	// ResourceCoverages maps percentage coverage per resource.
	ResourceCoverages map[string]float64
}

// CalculatePercentageValue calculates percentage value based on other fields.
func (c *CoverageValues) CalculatePercentageValue() {
	if c.TotalFields > 0 {
		c.PercentCoverage = (float64(c.CoveredFields) / float64(c.TotalFields-c.IgnoredFields)) * 100
	}
}

// GetAndRemoveResourceValue utility method to implement "get and delete"
// semantics. This makes templating operations easy.
func (c *CoveragePercentages) GetAndRemoveResourceValue(resource string) float64 {
	if resourcePercentage, ok := c.ResourceCoverages[resource]; ok {
		delete(c.ResourceCoverages, resource)
		return resourcePercentage
	}

	return 0.0
}

// IsFailedBuild utility method to indicate if CoveragePercentages indicate
// values of a failed build.
func (c *CoveragePercentages) IsFailedBuild() bool {
	return math.Abs(c.ResourceCoverages["Overall"]-0) == 0
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
