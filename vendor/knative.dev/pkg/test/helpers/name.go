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

package helpers

import (
	"strings"
	"testing"
)

const (
	testNamePrefix = "Test"
)

// ObjectPrefixForTest returns the name prefix for this test's random names.
func ObjectPrefixForTest(t *testing.T) string {
	return MakeK8sNamePrefix(strings.TrimPrefix(t.Name(), testNamePrefix))
}

// ObjectNameForTest generates a random object name based on the test name.
func ObjectNameForTest(t *testing.T) string {
	return AppendRandomString(ObjectPrefixForTest(t))
}
