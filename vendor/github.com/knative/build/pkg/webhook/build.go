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
	"context"
	"errors"
	"fmt"

	"github.com/mattbaird/jsonpatch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/build/pkg/apis/build/v1alpha1"
	"github.com/knative/build/pkg/logging"
)

func (ac *AdmissionController) validateBuild(ctx context.Context, _ *[]jsonpatch.JsonPatchOperation, old, new genericCRD) error {
	_, b, err := unmarshalBuilds(ctx, old, new)
	if err != nil {
		return err
	}

	if b.Spec.Template != nil && len(b.Spec.Steps) > 0 {
		return validationError("TemplateAndSteps", "build cannot specify both template and steps")
	}

	if b.Spec.Template != nil {
		if b.Spec.Template.Name == "" {
			return validationError("MissingTemplateName", "template instantiation is missing template name: %v", b.Spec.Template)
		}
	}

	// If a build specifies a template, all the template's parameters without
	// defaults must be satisfied by the build's parameters.
	var volumes []corev1.Volume
	var tmpl *v1alpha1.BuildTemplate
	if b.Spec.Template != nil {
		tmplName := b.Spec.Template.Name
		if tmplName == "" {
			return validationError("MissingTemplateName", "the build specifies a template without a name")
		}

		// Look up the template in the Build's namespace.
		tmpl, err = ac.buildClient.BuildV1alpha1().BuildTemplates(b.Namespace).Get(tmplName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if err := validateArguments(b.Spec.Template.Arguments, tmpl); err != nil {
			return err
		}
		volumes = tmpl.Spec.Volumes
	}
	if err := validateSteps(b.Spec.Steps); err != nil {
		return err
	}
	if err := validateVolumes(append(b.Spec.Volumes, volumes...)); err != nil {
		return err
	}

	// Do builder-implementation-specific validation.
	if err := ac.builder.Validate(b, tmpl); err != nil {
		return err
	}

	return nil
}

var errInvalidBuild = errors.New("failed to convert to Build")

func unmarshalBuilds(ctx context.Context, old, new genericCRD) (*v1alpha1.Build, *v1alpha1.Build, error) {
	logger := logging.FromContext(ctx)

	var oldb *v1alpha1.Build
	if old != nil {
		ok := false
		oldb, ok = old.(*v1alpha1.Build)
		if !ok {
			return nil, nil, errInvalidBuild
		}
	}
	logger.Infof("OLD Build is\n%+v", oldb)

	newbt, ok := new.(*v1alpha1.Build)
	if !ok {
		return nil, nil, errInvalidBuild
	}
	logger.Infof("NEW Build is\n%+v", newbt)

	return oldb, newbt, nil
}

func validateArguments(args []v1alpha1.ArgumentSpec, tmpl *v1alpha1.BuildTemplate) error {
	// Build must not duplicate argument names.
	seen := map[string]struct{}{}
	for _, a := range args {
		if _, ok := seen[a.Name]; ok {
			return validationError("DuplicateArgName", "duplicate argument name %q", a.Name)
		}
		seen[a.Name] = struct{}{}
	}
	// If a build specifies a template, all the template's parameters without
	// defaults must be satisfied by the build's parameters.
	if tmpl != nil {
		tmplParams := map[string]string{} // value is the param description.
		for _, p := range tmpl.Spec.Parameters {
			if p.Default == nil {
				tmplParams[p.Name] = p.Description
			}
		}
		for _, p := range args {
			delete(tmplParams, p.Name)
		}
		if len(tmplParams) > 0 {
			type pair struct{ name, desc string }
			var unused []pair
			for k, v := range tmplParams {
				unused = append(unused, pair{k, v})
			}
			return validationError("UnsatisfiedParameter", "build does not specify these required parameters: %s", unused)
		}
	}
	return nil
}

func validateSteps(steps []corev1.Container) error {
	// Build must not duplicate step names.
	names := map[string]struct{}{}
	for _, s := range steps {
		if s.Name == "" {
			continue
		}
		if _, ok := names[s.Name]; ok {
			return validationError("DuplicateStepName", "duplicate step name %q", s.Name)
		}
		names[s.Name] = struct{}{}
	}
	return nil
}

func validateVolumes(volumes []corev1.Volume) error {
	// Build must not duplicate volume names.
	vols := map[string]struct{}{}
	for _, v := range volumes {
		if _, ok := vols[v.Name]; ok {
			return validationError("DuplicateVolumeName", "duplicate volume name %q", v.Name)
		}
		vols[v.Name] = struct{}{}
	}
	return nil
}

type verror struct {
	reason, message string
}

func (ve *verror) Error() string { return fmt.Sprintf("%s: %s", ve.reason, ve.message) }

func validationError(reason, format string, fmtArgs ...interface{}) error {
	return &verror{
		reason:  reason,
		message: fmt.Sprintf(format, fmtArgs...),
	}
}
