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
	"sync"

	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource/aws"
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource/gcp"
)

// Interface is a type that represent monitor resource that satisfies monitoredresource.Interface
type Interface interface {
	// MonitoredResource returns the resource type and resource labels.
	MonitoredResource() (resType string, labels map[string]string)
}

// Autodetect auto detects monitored resources based on
// the environment where the application is running.
// It supports detection of following resource types
// 1. gke_container:
// 2. gce_instance:
// 3. aws_ec2_instance:
//
// Returns MonitoredResInterface which implements getLabels() and getType()
// For resource definition go to https://cloud.google.com/monitoring/api/resources
func Autodetect() Interface {
	return func() Interface {
		detectOnce.Do(func() {
			var awsDetect, gcpDetect Interface
			// First attempts to retrieve AWS Identity Doc and GCP metadata.
			// It then determines the resource type
			// In GCP and AWS environment both func finishes quickly. However,
			// in an environment other than those (e.g local laptop) it
			// takes 2 seconds for GCP and 5-6 for AWS.
			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				awsDetect = aws.Autodetect()
			}()
			go func() {
				defer wg.Done()
				gcpDetect = gcp.Autodetect()
			}()

			wg.Wait()
			autoDetected = awsDetect
			if gcpDetect != nil {
				autoDetected = gcpDetect
			}
		})
		return autoDetected
	}()
}

// detectOnce is used to make sure GCP and AWS metadata detect function executes only once.
var detectOnce sync.Once

// autoDetected is the metadata detected after the first execution of Autodetect function.
var autoDetected Interface
