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

package gcp

import (
	"os"
	"sync"
)

// Interface is a type that represent monitor resource that satisfies monitoredresource.Interface
type Interface interface {

	// MonitoredResource returns the resource type and resource labels.
	MonitoredResource() (resType string, labels map[string]string)
}

// GKEContainer represents gke_container type monitored resource.
// For definition refer to
// https://cloud.google.com/monitoring/api/resources#tag_gke_container
type GKEContainer struct {

	// ProjectID is the identifier of the GCP project associated with this resource, such as "my-project".
	ProjectID string

	// InstanceID is the numeric VM instance identifier assigned by Compute Engine.
	InstanceID string

	// ClusterName is the name for the cluster the container is running in.
	ClusterName string

	// ContainerName is the name of the container.
	ContainerName string

	// NamespaceID is the identifier for the cluster namespace the container is running in
	NamespaceID string

	// PodID is the identifier for the pod the container is running in.
	PodID string

	// Zone is the Compute Engine zone in which the VM is running.
	Zone string

	// LoggingMonitoringV2Enabled is the identifier if user enabled V2 logging and monitoring for GKE
	LoggingMonitoringV2Enabled bool
}

// MonitoredResource returns resource type and resource labels for GKEContainer
func (gke *GKEContainer) MonitoredResource() (resType string, labels map[string]string) {
	labels = map[string]string{
		"project_id":     gke.ProjectID,
		"cluster_name":   gke.ClusterName,
		"container_name": gke.ContainerName,
	}
	var typ string
	if gke.LoggingMonitoringV2Enabled {
		typ = "k8s_container"
		labels["pod_name"] = gke.PodID
		labels["namespace_name"] = gke.NamespaceID
		labels["location"] = gke.Zone
	} else {
		typ = "gke_container"
		labels["pod_id"] = gke.PodID
		labels["namespace_id"] = gke.NamespaceID
		labels["zone"] = gke.Zone
		labels["instance_id"] = gke.InstanceID
	}
	return typ, labels
}

// GCEInstance represents gce_instance type monitored resource.
// For definition refer to
// https://cloud.google.com/monitoring/api/resources#tag_gce_instance
type GCEInstance struct {

	// ProjectID is the identifier of the GCP project associated with this resource, such as "my-project".
	ProjectID string

	// InstanceID is the numeric VM instance identifier assigned by Compute Engine.
	InstanceID string

	// Zone is the Compute Engine zone in which the VM is running.
	Zone string
}

// MonitoredResource returns resource type and resource labels for GCEInstance
func (gce *GCEInstance) MonitoredResource() (resType string, labels map[string]string) {
	labels = map[string]string{
		"project_id":  gce.ProjectID,
		"instance_id": gce.InstanceID,
		"zone":        gce.Zone,
	}
	return "gce_instance", labels
}

// Autodetect auto detects monitored resources based on
// the environment where the application is running.
// It supports detection of following resource types
// 1. gke_container:
// 2. gce_instance:
//
// Returns MonitoredResInterface which implements getLabels() and getType()
// For resource definition go to https://cloud.google.com/monitoring/api/resources
func Autodetect() Interface {
	return func() Interface {
		detectOnce.Do(func() {
			autoDetected = detectResourceType(retrieveGCPMetadata())
		})
		return autoDetected
	}()

}

// createGCEInstanceMonitoredResource creates a gce_instance monitored resource
// gcpMetadata contains GCP (GKE or GCE) specific attributes.
func createGCEInstanceMonitoredResource(gcpMetadata *gcpMetadata) *GCEInstance {
	gceInstance := GCEInstance{
		ProjectID:  gcpMetadata.projectID,
		InstanceID: gcpMetadata.instanceID,
		Zone:       gcpMetadata.zone,
	}
	return &gceInstance
}

// createGKEContainerMonitoredResource creates a gke_container monitored resource
// gcpMetadata contains GCP (GKE or GCE) specific attributes.
func createGKEContainerMonitoredResource(gcpMetadata *gcpMetadata) *GKEContainer {
	gkeContainer := GKEContainer{
		ProjectID:                  gcpMetadata.projectID,
		InstanceID:                 gcpMetadata.instanceID,
		Zone:                       gcpMetadata.zone,
		ContainerName:              gcpMetadata.containerName,
		ClusterName:                gcpMetadata.clusterName,
		NamespaceID:                gcpMetadata.namespaceID,
		PodID:                      gcpMetadata.podID,
		LoggingMonitoringV2Enabled: gcpMetadata.monitoringV2,
	}
	return &gkeContainer
}

// detectOnce is used to make sure GCP metadata detect function executes only once.
var detectOnce sync.Once

// autoDetected is the metadata detected after the first execution of Autodetect function.
var autoDetected Interface

// detectResourceType determines the resource type.
// gcpMetadata contains GCP (GKE or GCE) specific attributes.
func detectResourceType(gcpMetadata *gcpMetadata) Interface {
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" &&
		gcpMetadata != nil && gcpMetadata.instanceID != "" {
		return createGKEContainerMonitoredResource(gcpMetadata)
	} else if gcpMetadata != nil && gcpMetadata.instanceID != "" {
		return createGCEInstanceMonitoredResource(gcpMetadata)
	}
	return nil
}
