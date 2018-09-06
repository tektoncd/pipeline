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

package convert

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/knative/build/pkg/builder/validation"
)

func ToEnvVarFromString(og string) (*corev1.EnvVar, error) {
	parts := strings.SplitN(og, "=", 2)
	if len(parts) != 2 {
		return nil, validation.NewError("MalformedEnv", "expected \"name=value\" form for entry, but got: %q", og)
	}
	return &corev1.EnvVar{
		Name:  parts[0],
		Value: parts[1],
	}, nil
}

func ToStringFromEnvVar(og *corev1.EnvVar) (string, error) {
	if og.ValueFrom != nil {
		return "", validation.NewError("ValueFrom", "container builder does not support the downward API, got: %v", og)
	}
	return fmt.Sprintf("%s=%s", og.Name, og.Value), nil
}

func ToEnvFromAssociativeList(og []string) ([]corev1.EnvVar, error) {
	al := make([]corev1.EnvVar, 0, len(og))
	for _, s := range og {
		envVar, err := ToEnvVarFromString(s)
		if err != nil {
			return nil, err
		}
		al = append(al, *envVar)
	}
	return al, nil
}

func ToAssociativeListFromEnv(og []corev1.EnvVar) ([]string, error) {
	al := make([]string, 0, len(og))
	for _, envVar := range og {
		s, err := ToStringFromEnvVar(&envVar)
		if err != nil {
			return nil, err
		}
		al = append(al, s)
	}
	return al, nil
}
