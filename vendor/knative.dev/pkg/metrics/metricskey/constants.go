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

package metricskey

import (
	"context"

	"go.opencensus.io/resource"
)

const (
	// LabelProject is the label for project (e.g. GCP GAIA ID, AWS project name)
	LabelProject = "project_id"

	// LabelLocation is the label for location (e.g. GCE zone, AWS region) where the service is deployed
	LabelLocation = "location"

	// LabelClusterName is the label for immutable name of the cluster
	LabelClusterName = "cluster_name"

	// LabelNamespaceName is the label for immutable name of the namespace that the service is deployed
	LabelNamespaceName = "namespace_name"

	// ContainerName is the container for which the metric is reported.
	ContainerName = "container_name"

	// PodName is the name of the pod for which the metric is reported.
	PodName = "pod_name"

	// LabelResponseCode is the label for the HTTP response status code.
	LabelResponseCode = "response_code"

	// LabelResponseCodeClass is the label for the HTTP response status code class. For example, "2xx", "3xx", etc.
	LabelResponseCodeClass = "response_code_class"

	// LabelResponseError is the label for client error. For HTTP, A non-2xx status code doesn't cause an error.
	LabelResponseError = "response_error"

	// LabelResponseTimeout is the label timeout.
	LabelResponseTimeout = "response_timeout"

	// ValueUnknown is the default value if the field is unknown, e.g. project will be unknown if Knative
	// is not running on GKE.
	ValueUnknown = "unknown"
)

type resourceKey struct{}

// WithResource associates the given monitoring Resource with the current
// context. Note that Resources do not "stack" or merge -- the closest enclosing
// Resource is the one that all measurements are associated with.
func WithResource(ctx context.Context, r resource.Resource) context.Context {
	return context.WithValue(ctx, resourceKey{}, &r)
}

// GetResource extracts a resource from the current context, if present.
func GetResource(ctx context.Context) *resource.Resource {
	r := ctx.Value(resourceKey{})
	if r == nil {
		return nil
	}
	return r.(*resource.Resource)
}
