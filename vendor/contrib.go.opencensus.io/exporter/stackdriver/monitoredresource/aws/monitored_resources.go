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

package aws

import (
	"fmt"
	"sync"
)

// Interface is a type that represent monitor resource that satisfies monitoredresource.Interface
type Interface interface {

	// MonitoredResource returns the resource type and resource labels.
	MonitoredResource() (resType string, labels map[string]string)
}

// EC2Instance represents aws_ec2_instance type monitored resource.
// For definition refer to
// https://cloud.google.com/monitoring/api/resources#tag_aws_ec2_instance
type EC2Instance struct {

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
func (aws *EC2Instance) MonitoredResource() (resType string, labels map[string]string) {
	labels = map[string]string{
		"aws_account": aws.AWSAccount,
		"instance_id": aws.InstanceID,
		"region":      aws.Region,
	}
	return "aws_ec2_instance", labels
}

// Autodetect auto detects monitored resources based on
// the environment where the application is running.
// It supports detection of following resource types
// 1. aws_ec2_instance:
//
// Returns MonitoredResInterface which implements getLabels() and getType()
// For resource definition go to https://cloud.google.com/monitoring/api/resources
func Autodetect() Interface {
	return func() Interface {
		detectOnce.Do(func() {
			autoDetected = detectResourceType(retrieveAWSIdentityDocument())
		})
		return autoDetected
	}()

}

// createAWSEC2InstanceMonitoredResource creates a aws_ec2_instance monitored resource
// awsIdentityDoc contains AWS EC2 specific attributes.
func createEC2InstanceMonitoredResource(awsIdentityDoc *awsIdentityDocument) *EC2Instance {
	awsInstance := EC2Instance{
		AWSAccount: awsIdentityDoc.accountID,
		InstanceID: awsIdentityDoc.instanceID,
		Region:     fmt.Sprintf("aws:%s", awsIdentityDoc.region),
	}
	return &awsInstance
}

// detectOnce is used to make sure AWS metadata detect function executes only once.
var detectOnce sync.Once

// autoDetected is the metadata detected after the first execution of Autodetect function.
var autoDetected Interface

// detectResourceType determines the resource type.
// awsIdentityDoc contains AWS EC2 attributes. nil if it is not AWS EC2 environment
func detectResourceType(awsIdentityDoc *awsIdentityDocument) Interface {
	if awsIdentityDoc != nil {
		return createEC2InstanceMonitoredResource(awsIdentityDoc)
	}
	return nil
}
