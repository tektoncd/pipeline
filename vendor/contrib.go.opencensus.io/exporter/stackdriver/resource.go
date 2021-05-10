// Copyright 2020, OpenCensus Authors
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

package stackdriver // import "contrib.go.opencensus.io/exporter/stackdriver"

import (
	"fmt"
	"sync"

	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource/gcp"
	"go.opencensus.io/resource"
	"go.opencensus.io/resource/resourcekeys"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

// Resource labels that are generally internal to the exporter.
// Consider exposing these labels and a type identifier in the future to allow
// for customization.
const (
	stackdriverProjectID            = "contrib.opencensus.io/exporter/stackdriver/project_id"
	stackdriverLocation             = "contrib.opencensus.io/exporter/stackdriver/location"
	stackdriverClusterName          = "contrib.opencesus.io/exporter/stackdriver/cluster_name"
	stackdriverGenericTaskNamespace = "contrib.opencensus.io/exporter/stackdriver/generic_task/namespace"
	stackdriverGenericTaskJob       = "contrib.opencensus.io/exporter/stackdriver/generic_task/job"
	stackdriverGenericTaskID        = "contrib.opencensus.io/exporter/stackdriver/generic_task/task_id"

	knativeResType           = "knative_revision"
	knativeServiceName       = "service_name"
	knativeRevisionName      = "revision_name"
	knativeConfigurationName = "configuration_name"
	knativeNamespaceName     = "namespace_name"

	appEngineInstanceType = "gae_instance"

	appEngineService  = "appengine.service.id"
	appEngineVersion  = "appengine.version.id"
	appEngineInstance = "appengine.instance.id"
)

var (
	// autodetectFunc returns a monitored resource that is autodetected.
	// from the cloud environment at runtime.
	autodetectFunc func() gcp.Interface

	// autodetectOnce is used to lazy initialize autodetectedLabels.
	autodetectOnce *sync.Once
	// autodetectedLabels stores all the labels from the autodetected monitored resource
	// with a possible additional label for the GCP "location".
	autodetectedLabels map[string]string
)

func init() {
	autodetectFunc = gcp.Autodetect
	// monitoredresource.Autodetect only makes calls to the metadata APIs once
	// and caches the results
	autodetectOnce = new(sync.Once)
}

// Mappings for the well-known OpenCensus resource label keys
// to applicable Stackdriver Monitored Resource label keys.
var k8sContainerMap = map[string]string{
	"project_id":     stackdriverProjectID,
	"location":       resourcekeys.CloudKeyZone,
	"cluster_name":   resourcekeys.K8SKeyClusterName,
	"namespace_name": resourcekeys.K8SKeyNamespaceName,
	"pod_name":       resourcekeys.K8SKeyPodName,
	"container_name": resourcekeys.ContainerKeyName,
}

var k8sPodMap = map[string]string{
	"project_id":     stackdriverProjectID,
	"location":       resourcekeys.CloudKeyZone,
	"cluster_name":   resourcekeys.K8SKeyClusterName,
	"namespace_name": resourcekeys.K8SKeyNamespaceName,
	"pod_name":       resourcekeys.K8SKeyPodName,
}

var k8sNodeMap = map[string]string{
	"project_id":   stackdriverProjectID,
	"location":     resourcekeys.CloudKeyZone,
	"cluster_name": resourcekeys.K8SKeyClusterName,
	"node_name":    resourcekeys.HostKeyName,
}

var gcpResourceMap = map[string]string{
	"project_id":  stackdriverProjectID,
	"instance_id": resourcekeys.HostKeyID,
	"zone":        resourcekeys.CloudKeyZone,
}

var awsResourceMap = map[string]string{
	"project_id":  stackdriverProjectID,
	"instance_id": resourcekeys.HostKeyID,
	"region":      resourcekeys.CloudKeyRegion,
	"aws_account": resourcekeys.CloudKeyAccountID,
}

var appEngineInstanceMap = map[string]string{
	"project_id":  stackdriverProjectID,
	"location":    resourcekeys.CloudKeyRegion,
	"module_id":   appEngineService,
	"version_id":  appEngineVersion,
	"instance_id": appEngineInstance,
}

// Generic task resource.
var genericResourceMap = map[string]string{
	"project_id": stackdriverProjectID,
	"location":   resourcekeys.CloudKeyZone,
	"namespace":  stackdriverGenericTaskNamespace,
	"job":        stackdriverGenericTaskJob,
	"task_id":    stackdriverGenericTaskID,
}

var knativeRevisionResourceMap = map[string]string{
	"project_id":             stackdriverProjectID,
	"location":               resourcekeys.CloudKeyZone,
	"cluster_name":           resourcekeys.K8SKeyClusterName,
	knativeServiceName:       knativeServiceName,
	knativeRevisionName:      knativeRevisionName,
	knativeConfigurationName: knativeConfigurationName,
	knativeNamespaceName:     knativeNamespaceName,
}

// getAutodetectedLabels returns all the labels from the Monitored Resource detected
// from the environment by calling monitoredresource.Autodetect. If a "zone" label is detected,
// a "location" label is added with the same value to account for differences between
// Legacy Stackdriver and Stackdriver Kubernetes Engine Monitoring,
// see https://cloud.google.com/monitoring/kubernetes-engine/migration.
func getAutodetectedLabels() map[string]string {
	autodetectOnce.Do(func() {
		autodetectedLabels = map[string]string{}
		if mr := autodetectFunc(); mr != nil {
			_, labels := mr.MonitoredResource()
			// accept "zone" value for "location" because values for location can be a zone
			// or region, see https://cloud.google.com/docs/geography-and-regions
			if _, ok := labels["zone"]; ok {
				labels["location"] = labels["zone"]
			}

			autodetectedLabels = labels
		}
	})

	return autodetectedLabels
}

// returns transformed label map and true if all labels in match are found
// in input except optional project_id. It returns false if at least one label
// other than project_id is missing.
func transformResource(match, input map[string]string) (map[string]string, bool) {
	output := make(map[string]string, len(input))
	for dst, src := range match {
		if v, ok := input[src]; ok {
			output[dst] = v
			continue
		}

		// attempt to autodetect missing labels, autodetected label keys should
		// match destination label keys
		if v, ok := getAutodetectedLabels()[dst]; ok {
			output[dst] = v
			continue
		}

		if dst != "project_id" {
			return nil, true
		}
	}
	return output, false
}

// DefaultMapResource implements default resource mapping for well-known resource types
func DefaultMapResource(res *resource.Resource) *monitoredrespb.MonitoredResource {
	match := genericResourceMap
	result := &monitoredrespb.MonitoredResource{
		Type: "global",
	}
	if res == nil || res.Labels == nil {
		return result
	}

	switch {
	case res.Type == resourcekeys.ContainerType:
		result.Type = "k8s_container"
		match = k8sContainerMap
	case res.Type == resourcekeys.K8SType:
		result.Type = "k8s_pod"
		match = k8sPodMap
	case res.Type == resourcekeys.HostType && res.Labels[resourcekeys.K8SKeyClusterName] != "":
		result.Type = "k8s_node"
		match = k8sNodeMap
	case res.Type == appEngineInstanceType:
		result.Type = appEngineInstanceType
		match = appEngineInstanceMap
	case res.Labels[resourcekeys.CloudKeyProvider] == resourcekeys.CloudProviderGCP:
		result.Type = "gce_instance"
		match = gcpResourceMap
	case res.Labels[resourcekeys.CloudKeyProvider] == resourcekeys.CloudProviderAWS:
		result.Type = "aws_ec2_instance"
		match = awsResourceMap
	case res.Type == knativeResType:
		result.Type = res.Type
		match = knativeRevisionResourceMap
	}

	var missing bool
	result.Labels, missing = transformResource(match, res.Labels)
	if missing {
		result.Type = "global"
		// if project id specified then transform it.
		if v, ok := res.Labels[stackdriverProjectID]; ok {
			result.Labels = make(map[string]string, 1)
			result.Labels["project_id"] = v
		}
		return result
	}
	if result.Type == "aws_ec2_instance" {
		if v, ok := result.Labels["region"]; ok {
			result.Labels["region"] = fmt.Sprintf("aws:%s", v)
		}
	}
	return result
}
