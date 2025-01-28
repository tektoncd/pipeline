/*
Copyright 2020 The Knative Authors

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

package webhook

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/webhook/certificates/resources"
)

// EnsureLabelSelectorExpressions merges the current label selector's MatchExpressions
// with the ones wanted.
// It keeps all non-knative keys intact, removes all knative-keys no longer wanted and
// adds all knative-keys not yet there.
func EnsureLabelSelectorExpressions(
	current *metav1.LabelSelector,
	want *metav1.LabelSelector,
) *metav1.LabelSelector {
	if current == nil {
		return want
	}

	if len(current.MatchExpressions) == 0 {
		return &metav1.LabelSelector{
			MatchLabels:      current.MatchLabels,
			MatchExpressions: want.MatchExpressions,
		}
	}

	var wantExpressions []metav1.LabelSelectorRequirement
	if want != nil {
		wantExpressions = want.MatchExpressions
	}

	return &metav1.LabelSelector{
		MatchLabels: current.MatchLabels,
		MatchExpressions: ensureLabelSelectorRequirements(
			current.MatchExpressions, wantExpressions),
	}
}

func ensureLabelSelectorRequirements(
	current []metav1.LabelSelectorRequirement,
	want []metav1.LabelSelectorRequirement,
) []metav1.LabelSelectorRequirement {
	nonKnative := make([]metav1.LabelSelectorRequirement, 0, len(current))
	for _, r := range current {
		if !strings.Contains(r.Key, "knative.dev") {
			nonKnative = append(nonKnative, r)
		}
	}

	return append(want, nonKnative...)
}

func getSecretDataKeyNamesOrDefault(sKey string, sCert string) (serverKey string, serverCert string) {
	serverKey = resources.ServerKey
	serverCert = resources.ServerCert

	if sKey != "" {
		serverKey = sKey
	}
	if sCert != "" {
		serverCert = sCert
	}
	return serverKey, serverCert
}
