/*
Copyright 2018 The Knative Authors

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

// testgrid.go provides methods to perform action on testgrid.

package main

import (
	"encoding/xml"
	"log"
	"os"
)

const (
	apiCoverage    = "api_coverage"
	overallRoute   = "OverallRoute"
	overallConfig  = "OverallConfiguration"
	overallService = "OverallService"
)

// TestProperty defines a property of the test
type TestProperty struct {
	Name  string  `xml:"name,attr"`
	Value float32 `xml:"value,attr"`
}

// TestProperties is an array of test properties
type TestProperties struct {
	Property []TestProperty `xml:"property"`
}

// TestCase defines a test case that was executed
type TestCase struct {
	ClassName  string         `xml:"class_name,attr"`
	Name       string         `xml:"name,attr"`
	Time       int            `xml:"time,attr"`
	Properties TestProperties `xml:"properties"`
	Fail       bool           `xml:"failure,omitempty"`
}

// TestSuite defines the set of relevant test cases
type TestSuite struct {
	XMLName   xml.Name   `xml:"testsuite"`
	TestCases []TestCase `xml:"testcase"`
}

func createTestProperty(value float32) TestProperty {
	return TestProperty{Name: apiCoverage, Value: value}
}

func createTestCase(name string, properties []TestProperty, fail bool) TestCase {
	tc := TestCase{ClassName: apiCoverage, Name: name, Properties: TestProperties{Property: properties}, Time: 0}
	if fail {
		tc.Fail = true
	}
	return tc
}

func createTestSuite(cases []TestCase) TestSuite {
	return TestSuite{TestCases: cases}
}

func createCases(tcName string, covered map[string]int, notCovered map[string]int) []TestCase {
	var tc []TestCase

	var percentCovered = float32(100 * len(covered) / (len(covered) + len(notCovered)))
	tp := []TestProperty{createTestProperty(percentCovered)}
	tc = append(tc, createTestCase(tcName, tp, false))

	for key, value := range covered {
		tp := []TestProperty{createTestProperty(float32(value))}
		tc = append(tc, createTestCase(tcName+"/"+key, tp, false))
	}

	for key, value := range notCovered {
		tp := []TestProperty{createTestProperty(float32(value))}
		tc = append(tc, createTestCase(tcName+"/"+key, tp, true))
	}
	return tc
}

func createTestgridXML(coverage *OverallAPICoverage, artifactsDir string) {
	tc := createCases(overallRoute, coverage.RouteAPICovered, coverage.RouteAPINotCovered)
	tc = append(tc, createCases(overallConfig, coverage.ConfigurationAPICovered, coverage.ConfigurationAPINotCovered)...)
	tc = append(tc, createCases(overallService, coverage.ServiceAPICovered, coverage.ServiceAPINotCovered)...)
	ts := createTestSuite(tc)

	op, err := xml.MarshalIndent(ts, "", "  ")
	if err != nil {
		log.Fatalf("Error creating xml: %v", err)
	}

	outputFile := artifactsDir + "/junit_bazel.xml"
	f, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("Cannot create '%s': %v", outputFile, err)
	}
	defer f.Close()
	if _, err := f.WriteString(string(op) + "\n"); err != nil {
		log.Fatalf("Cannot write to '%s': %v", f.Name(), err)
	}
}
