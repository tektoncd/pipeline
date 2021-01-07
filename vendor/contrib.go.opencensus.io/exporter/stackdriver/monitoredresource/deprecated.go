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

package monitoredresource

import (
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource/aws"
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource/gcp"
)

// GKEContainer represents gke_container type monitored resource.
// For definition refer to
// https://cloud.google.com/monitoring/api/resources#tag_gke_container
// Deprecated: please use gcp.GKEContainer from "contrib.go.opencensus.io/exporter/stackdriver/monitoredresource/gcp".
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
	gcpGKE := gcp.GKEContainer(*gke)
	return gcpGKE.MonitoredResource()
}

// GCEInstance represents gce_instance type monitored resource.
// For definition refer to
// https://cloud.google.com/monitoring/api/resources#tag_gce_instance
// Deprecated: please use gcp.GCEInstance from "contrib.go.opencensus.io/exporter/stackdriver/monitoredresource/gcp".
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
	gcpGCE := gcp.GCEInstance(*gce)
	return gcpGCE.MonitoredResource()
}

// AWSEC2Instance represents aws_ec2_instance type monitored resource.
// For definition refer to
// https://cloud.google.com/monitoring/api/resources#tag_aws_ec2_instance
// Deprecated: please use aws.EC2Container from "contrib.go.opencensus.io/exporter/stackdriver/monitoredresource/aws".
type AWSEC2Instance struct {
	// AWSAccount is the AWS account number for the VM.
	AWSAccount string

	// InstanceID is the instance id of the instance.
	InstanceID string

	// Region is the AWS region for the VM. The format of this field is "aws:{region}",
	// where supported values for {region} are listed at
	// http://docs.aws.amazon.com/general/latest/gr/rande.html.
	Region string
}

// MonitoredResource returns resource type and resource labels for AWSEC2Instance
func (ec2 *AWSEC2Instance) MonitoredResource() (resType string, labels map[string]string) {
	awsEC2 := aws.EC2Instance(*ec2)
	return awsEC2.MonitoredResource()
}
