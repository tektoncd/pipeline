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

	"github.com/mattbaird/jsonpatch"

	"github.com/knative/build/pkg/apis/build/v1alpha1"
	"github.com/knative/pkg/logging"
)

func (ac *AdmissionController) validateBuildTemplate(ctx context.Context, _ *[]jsonpatch.JsonPatchOperation, old, new genericCRD) error {
	_, tmpl, err := unmarshalBuildTemplates(ctx, old, new)
	if err != nil {
		return err
	}
	if err := validateTemplate(tmpl); err != nil {
		return err
	}
	return nil
}

var errInvalidBuildTemplate = errors.New("failed to convert to BuildTemplate")

func unmarshalBuildTemplates(ctx context.Context, old, new genericCRD) (*v1alpha1.BuildTemplate, *v1alpha1.BuildTemplate, error) {
	logger := logging.FromContext(ctx)

	var oldbt *v1alpha1.BuildTemplate
	if old != nil {
		ok := false
		oldbt, ok = old.(*v1alpha1.BuildTemplate)
		if !ok {
			return nil, nil, errInvalidBuildTemplate
		}
	}
	logger.Infof("OLD BuildTemplate is\n%+v", oldbt)

	newbt, ok := new.(*v1alpha1.BuildTemplate)
	if !ok {
		return nil, nil, errInvalidBuildTemplate
	}
	logger.Infof("NEW BuildTemplate is\n%+v", newbt)

	return oldbt, newbt, nil
}
