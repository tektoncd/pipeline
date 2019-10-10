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

const (
	// LabelProject is the label for project (e.g. GCP GAIA ID, AWS project name)
	LabelProject = "project_id"

	// LabelLocation is the label for location (e.g. GCE zone, AWS region) where the service is deployed
	LabelLocation = "location"

	// LabelClusterName is the label for immutable name of the cluster
	LabelClusterName = "cluster_name"

	// LabelNamespaceName is the label for immutable name of the namespace that the service is deployed
	LabelNamespaceName = "namespace_name"

	// ValueUnknown is the default value if the field is unknown, e.g. project will be unknown if Knative
	// is not running on GKE.
	ValueUnknown = "unknown"
)
