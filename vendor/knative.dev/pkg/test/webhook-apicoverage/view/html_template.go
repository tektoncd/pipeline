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

var TypeCoverageTempl = fmt.Sprint(`<!DOCTYPE html>
<html>
<style type="text/css">
  <!--

  .tab { margin-left: 50px; }

  .styleheader {color: white; size: A4}

  .covered {color: green; size: A3}

  .notcovered {color: red; size: A3}

  .ignored {color: white; size: A4}

  .values {color: yellow; size: A3}

  table, th, td { border: 1px solid white; text-align: center}

  .braces {color: white; size: A3}
  -->
</style>
<body style="background-color:rgb(0,0,0); font-family: Arial">
{{ range $coverageType := .TypeCoverages }}
  <div class="styleheader">
    <br>Package: {{ $coverageType.Package }}
    <br>Type: {{ $coverageType.Type }}
    <br>
    <div class="braces">
      <br>{
    </div>
    {{ range $key, $value := $coverageType.Fields }}
      {{if $value.Ignored }}
        <div class="ignored tab">{{ $value.Field }}</div>
      {{else if $value.Coverage}}
        <div class="covered tab">{{ $value.Field }}
          {{ $valueLen := len $value.Values }}
          {{if gt $valueLen 0 }}
            &emsp; &emsp; <span class="values">Values: [{{$value.GetValuesForDisplay}}]</span>
          {{end}}
        </div>
      {{else}}
        <div class="notcovered tab">{{ $value.Field }}</div>
      {{end}}
    {{end}}
    <div class="braces">}</div>
  </div>
{{end}}

<br>
<br>
<div class="styleheader">Coverage Values</div>
<br>
<table style="width: 30%">
  <tr class="styleheader"><td>Total Fields</td><td>{{ .CoverageNumbers.TotalFields }}</td></tr>
  <tr class="styleheader"><td>Covered Fields</td><td>{{ .CoverageNumbers.CoveredFields }}</td></tr>
  <tr class="styleheader"><td>Ignored Fields</td><td>{{ .CoverageNumbers.IgnoredFields }}</td></tr>
  <tr class="styleheader"><td>Coverage Percentage</td><td>{{ .CoverageNumbers.PercentCoverage }}</td></tr>
</table>
</body>
</html>
`)

var AggregateCoverageTmpl = fmt.Sprint(`<!DOCTYPE html>
<html>
<style type="text/css">
  <!--

  .tab { margin-left: 50px; }

  .styleheader {color: white; size: A4}

  .values {color: yellow; size: A3}

  table, th, td { border: 1px solid white; text-align: center}

  .braces {color: white; size: A3}
  -->
</style>
<body style="background-color:rgb(0,0,0); font-family: Arial">
<table style="width: 30%">
  <tr class="styleheader"><td>Total Fields</td><td>{{ .TotalFields }}</td></tr>
  <tr class="styleheader"><td>Covered Fields</td><td>{{ .CoveredFields }}</td></tr>
  <tr class="styleheader"><td>Ignored Fields</td><td>{{ .IgnoredFields }}</td></tr>
  <tr class="styleheader"><td>Coverage Percentage</td><td>{{ .PercentCoverage }}</td></tr>
</table>
</body>
</html>
`)
