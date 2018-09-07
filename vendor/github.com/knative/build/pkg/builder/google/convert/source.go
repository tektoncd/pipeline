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
	"strconv"
	"strings"

	v1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	cloudbuild "google.golang.org/api/cloudbuild/v1"

	"github.com/knative/build/pkg/builder/validation"
)

func ToGCSFromStorageSource(og *cloudbuild.StorageSource) *v1alpha1.GCSSourceSpec {
	loc := fmt.Sprintf("gs://%s/%s", og.Bucket, og.Object)
	if og.Generation != 0 {
		loc += fmt.Sprintf("#%d", og.Generation)
	}
	return &v1alpha1.GCSSourceSpec{
		Type:     v1alpha1.GCSArchive,
		Location: loc,
	}
}

func ToStorageSourceFromGCS(og *v1alpha1.GCSSourceSpec) (*cloudbuild.StorageSource, error) {
	if og.Type != v1alpha1.GCSArchive {
		return nil, validation.NewError("UnsupportedSource", "only GCS archive source is supported, got %q", og.Type)
	}
	loc := og.Location
	if !strings.HasPrefix(loc, "gs://") {
		return nil, validation.NewError("UnsupportedSource", `GCS source must begin with "gs://", got %q`, loc)
	}
	parts := strings.SplitN(strings.TrimPrefix(loc, "gs://"), "/", 2)
	if len(parts) < 2 {
		return nil, validation.NewError("MalformedLocation", "GCS source must specify bucket and object, got %q", loc)
	}
	bucket := parts[0]
	parts = strings.SplitN(parts[1], "#", 2)
	object := parts[0]
	var generation int64
	if len(parts) > 1 {
		var err error
		generation, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, validation.NewError("MalformedLocation", "Unable to parse GCS object generation %q: %v", parts[1], err)
		}
	}
	return &cloudbuild.StorageSource{
		Bucket:     bucket,
		Object:     object,
		Generation: generation,
	}, nil
}
