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
	"fmt"
)

var JunitResultTmpl = fmt.Sprint(`<testsuites>
  <testsuite name="" time="0" {{ if .IsFailedBuild }} failures="1" {{ else }} failures = "0" {{ end }} tests="0">
      <testcase name="Overall" time="0" classname="go_coverage">
				{{ if .IsFailedBuild }}
					<failure>true</failure>
        {{ end }}
				<properties>
          <property name="coverage" value="{{ .GetAndRemoveResourceValue "Overall" }}"/>
        </properties>
      </testcase>
    {{ range $key, $value := .ResourceCoverages }}
      <testcase name="{{ $key }}" time="0" classname="go_coverage">
        <properties>
          <property name="coverage" value="{{ $value }}"/>
        </properties>
      </testcase>
    {{end}}
  </testsuite>
</testsuites>`)
