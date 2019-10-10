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

import "knative.dev/pkg/test/webhook-apicoverage/coveragecalculator"

// DisplayRules provides a mechanism for repos to define their own display rules.
// DisplayHelper methods can use these rules to define how to display results.
type DisplayRules struct {
	PackageNameRule func(packageName string) string
	TypeNameRule    func(typeName string) string
	FieldRule       func(coverage *coveragecalculator.FieldCoverage) string
}
