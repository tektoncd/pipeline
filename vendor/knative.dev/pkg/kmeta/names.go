/*
copyright 2019 the knative authors

licensed under the apache license, version 2.0 (the "license");
you may not use this file except in compliance with the license.
you may obtain a copy of the license at

    http://www.apache.org/licenses/license-2.0

unless required by applicable law or agreed to in writing, software
distributed under the license is distributed on an "as is" basis,
without warranties or conditions of any kind, either express or implied.
see the license for the specific language governing permissions and
limitations under the license.
*/

package kmeta

import (
	"crypto/md5"
	"fmt"
)

// The longest name supported by the K8s is 63.
// These constants
const (
	longest = 63
	md5Len  = 32
	head    = longest - md5Len
)

// ChildName generates a name for the resource based upong the parent resource and suffix.
// If the concatenated name is longer than K8s permits the name is hashed and truncated to permit
// construction of the resource, but still keeps it unique.
func ChildName(parent, suffix string) string {
	n := parent
	if len(parent) > (longest - len(suffix)) {
		n = fmt.Sprintf("%s%x", parent[:head-len(suffix)], md5.Sum([]byte(parent)))
	}
	return n + suffix
}
