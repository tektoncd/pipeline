// Copyright © 2019 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package formatted

import (
	"github.com/hako/durafmt"
	"github.com/jonboulle/clockwork"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Age(t *metav1.Time, c clockwork.Clock) string {
	if t.IsZero() {
		return "---"
	}

	dur := c.Since(t.Time)
	return durafmt.ParseShort(dur).String() + " ago"
}

func Duration(t1, t2 *metav1.Time) string {
	if t1.IsZero() || t2.IsZero() {
		return "---"
	}

	dur := t2.Time.Sub(t1.Time)
	return durafmt.ParseShort(dur).String()
}
