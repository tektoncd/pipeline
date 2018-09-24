/*
Copyright 2017 Google Inc. All Rights Reserved.
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
	"regexp"

	corev1 "k8s.io/api/core/v1"

	"github.com/knative/build/pkg/apis/build/v1alpha1"
)

var nestedPlaceholderRE = regexp.MustCompile(`\${[^}]+\$`)

func validateTemplate(tmpl v1alpha1.BuildTemplateInterface) error {
	if err := validateSteps(tmpl.TemplateSpec().Steps); err != nil {
		return err
	}
	if err := validateVolumes(tmpl.TemplateSpec().Volumes); err != nil {
		return err
	}
	if err := validateParameters(tmpl.TemplateSpec().Parameters); err != nil {
		return err
	}
	return validatePlaceholders(tmpl.TemplateSpec().Steps)
}

func validateParameters(params []v1alpha1.ParameterSpec) error {
	// Template must not duplicate parameter names.
	seen := map[string]struct{}{}
	for _, p := range params {
		if _, ok := seen[p.Name]; ok {
			return validationError("DuplicateParamName", "duplicate template parameter name %q", p.Name)
		}
		seen[p.Name] = struct{}{}
	}
	return nil
}

func validatePlaceholders(steps []corev1.Container) error {
	for si, s := range steps {
		if nestedPlaceholderRE.MatchString(s.Name) {
			return validationError("NestedPlaceholder", "nested placeholder in step name %d: %q", si, s.Name)
		}
		for i, a := range s.Args {
			if nestedPlaceholderRE.MatchString(a) {
				return validationError("NestedPlaceholder", "nested placeholder in step %d arg %d: %q", si, i, a)
			}
		}
		for i, e := range s.Env {
			if nestedPlaceholderRE.MatchString(e.Value) {
				return validationError("NestedPlaceholder", "nested placeholder in step %d env value %d: %q", si, i, e.Value)
			}
		}
		if nestedPlaceholderRE.MatchString(s.WorkingDir) {
			return validationError("NestedPlaceholder", "nested placeholder in step %d working dir %q", si, s.WorkingDir)
		}
		for i, c := range s.Command {
			if nestedPlaceholderRE.MatchString(c) {
				return validationError("NestedPlaceholder", "nested placeholder in step %d command %d: %q", si, i, c)
			}
		}
	}
	return nil
}
