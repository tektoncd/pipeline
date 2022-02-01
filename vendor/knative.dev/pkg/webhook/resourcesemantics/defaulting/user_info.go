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

package defaulting

import (
	"context"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"knative.dev/pkg/apis"
)

var (
	emptyGroupUpdaterAnnotation = apis.UpdaterAnnotationSuffix[1:]
	emptyGroupCreatorAnnotation = apis.CreatorAnnotationSuffix[1:]
)

// setUserInfoAnnotations sets creator and updater annotations on a resource.
func setUserInfoAnnotations(ctx context.Context, resource apis.HasSpec, groupName string) {
	if ui := apis.GetUserInfo(ctx); ui != nil {
		objectMetaAccessor, ok := resource.(metav1.ObjectMetaAccessor)
		if !ok {
			return
		}

		annotations := objectMetaAccessor.GetObjectMeta().GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
			objectMetaAccessor.GetObjectMeta().SetAnnotations(annotations)
		}

		updaterAnnotation := emptyGroupUpdaterAnnotation
		creatorAnnotation := emptyGroupCreatorAnnotation
		if groupName != "" {
			updaterAnnotation = groupName + apis.UpdaterAnnotationSuffix
			creatorAnnotation = groupName + apis.CreatorAnnotationSuffix
		}

		if apis.IsInUpdate(ctx) {
			old := apis.GetBaseline(ctx).(apis.HasSpec)
			if equality.Semantic.DeepEqual(old.GetUntypedSpec(), resource.GetUntypedSpec()) {
				return
			}
			annotations[updaterAnnotation] = ui.Username
		} else {
			annotations[creatorAnnotation] = ui.Username
			annotations[updaterAnnotation] = ui.Username
		}
		objectMetaAccessor.GetObjectMeta().SetAnnotations(annotations)
	}
}

type unstructuredHasSpec struct {
	*unstructured.Unstructured
}

func (us unstructuredHasSpec) GetObjectMeta() metav1.Object {
	return us.Unstructured
}

var _ metav1.ObjectMetaAccessor = unstructuredHasSpec{}

func (us unstructuredHasSpec) GetUntypedSpec() interface{} {
	if s, ok := us.Unstructured.Object["spec"]; ok {
		return s
	}
	return nil
}

func adaptUnstructuredHasSpecCtx(ctx context.Context, req *admissionv1.AdmissionRequest) context.Context {
	if apis.IsInUpdate(ctx) {
		b := apis.GetBaseline(ctx)
		if apis.IsInStatusUpdate(ctx) {
			ctx = apis.WithinSubResourceUpdate(ctx, unstructuredHasSpec{b.(*unstructured.Unstructured)}, req.SubResource)
		} else {
			ctx = apis.WithinUpdate(ctx, unstructuredHasSpec{b.(*unstructured.Unstructured)})
		}
	}
	return ctx
}
