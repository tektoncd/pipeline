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
	"math/rand"
	"strings"
	"time"
	"unicode"
)

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyz"
	randSuffixLen = 8
	sep           = '-'
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// AppendRandomString will generate a random string that begins with prefix.
// This is useful if you want to make sure that your tests can run at the same
// time against the same environment without conflicting.
// This method will use "-" as the separator between the prefix and
// the random suffix.
// This method will seed rand with the current time when the package is initialized.
func AppendRandomString(prefix string) string {
	suffix := make([]byte, randSuffixLen)

	for i := range suffix {
		suffix[i] = letterBytes[rand.Intn(len(letterBytes))]
	}

	return strings.Join([]string{prefix, string(suffix)}, string(sep))
}

// MakeK8sNamePrefix converts each chunk of non-alphanumeric character into a single dash
// and also convert camelcase tokens into dash-delimited lowercase tokens.
func MakeK8sNamePrefix(s string) string {
	var sb strings.Builder
	newToken := false
	for _, c := range s {
		if !(unicode.IsLetter(c) || unicode.IsNumber(c)) {
			newToken = true
			continue
		}
		if sb.Len() > 0 && (newToken || unicode.IsUpper(c)) {
			sb.WriteRune(sep)
		}
		sb.WriteRune(unicode.ToLower(c))
		newToken = false
	}
	return sb.String()
}

// GetBaseFuncName returns the baseFuncName parsed from the fullFuncName.
// eg. test/e2e.TestMain will return TestMain.
func GetBaseFuncName(fullFuncName string) string {
	baseFuncName := fullFuncName[strings.LastIndex(fullFuncName, "/")+1:]
	baseFuncName = baseFuncName[strings.LastIndex(baseFuncName, ".")+1:]
	return baseFuncName
}
