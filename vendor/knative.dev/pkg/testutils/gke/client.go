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

package gke

import (
	"fmt"

	container "google.golang.org/api/container/v1beta1"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
)

// SDKOperations wraps GKE SDK related functions
type SDKOperations interface {
	CreateCluster(string, string, string, *container.CreateClusterRequest) error
	CreateClusterAsync(string, string, string, *container.CreateClusterRequest) (*container.Operation, error)
	DeleteCluster(string, string, string, string) error
	DeleteClusterAsync(string, string, string, string) (*container.Operation, error)
	GetCluster(string, string, string, string) (*container.Cluster, error)
	GetOperation(string, string, string, string) (*container.Operation, error)
	ListClustersInProject(string) ([]*container.Cluster, error)
}

// sdkClient Implement SDKOperations
type sdkClient struct {
	*container.Service
}

// NewSDKClient returns an SDKClient that implements SDKOperations
func NewSDKClient() (SDKOperations, error) {
	ctx := context.Background()
	c, err := google.DefaultClient(ctx, container.CloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("failed to create Google client: '%v'", err)
	}

	containerService, err := container.New(c)
	if err != nil {
		return nil, fmt.Errorf("failed to create container service: '%v'", err)
	}
	return &sdkClient{containerService}, nil
}

// CreateCluster creates a new GKE cluster, and wait until it finishes or timeout or there is an error.
func (gsc *sdkClient) CreateCluster(
	project, region, zone string,
	rb *container.CreateClusterRequest,
) error {
	op, err := gsc.CreateClusterAsync(project, region, zone, rb)
	if err == nil {
		err = Wait(gsc, project, region, zone, op.Name, creationTimeout)
	}
	return err
}

// CreateClusterAsync creates a new GKE cluster asynchronously.
func (gsc *sdkClient) CreateClusterAsync(
	project, region, zone string,
	rb *container.CreateClusterRequest,
) (*container.Operation, error) {
	location := GetClusterLocation(region, zone)
	if zone != "" {
		return gsc.Projects.Zones.Clusters.Create(project, location, rb).Context(context.Background()).Do()
	}
	parent := fmt.Sprintf("projects/%s/locations/%s", project, location)
	return gsc.Projects.Locations.Clusters.Create(parent, rb).Context(context.Background()).Do()
}

// DeleteCluster deletes the GKE cluster, and wait until it finishes or timeout or there is an error.
func (gsc *sdkClient) DeleteCluster(project, region, zone, clusterName string) error {
	op, err := gsc.DeleteClusterAsync(project, region, zone, clusterName)
	if err == nil {
		err = Wait(gsc, project, region, zone, op.Name, deletionTimeout)
	}
	return err
}

// DeleteClusterAsync deletes the GKE cluster asynchronously.
func (gsc *sdkClient) DeleteClusterAsync(project, region, zone, clusterName string) (*container.Operation, error) {
	location := GetClusterLocation(region, zone)
	if zone != "" {
		return gsc.Projects.Zones.Clusters.Delete(project, location, clusterName).Context(context.Background()).Do()
	}
	clusterFullPath := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", project, location, clusterName)
	return gsc.Projects.Locations.Clusters.Delete(clusterFullPath).Context(context.Background()).Do()
}

// GetCluster gets the GKE cluster with the given cluster name.
func (gsc *sdkClient) GetCluster(project, region, zone, clusterName string) (*container.Cluster, error) {
	location := GetClusterLocation(region, zone)
	if zone != "" {
		return gsc.Projects.Zones.Clusters.Get(project, location, clusterName).Context(context.Background()).Do()
	}
	clusterFullPath := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", project, location, clusterName)
	return gsc.Projects.Locations.Clusters.Get(clusterFullPath).Context(context.Background()).Do()
}

// ListClustersInProject lists all the GKE clusters created in the given project.
func (gsc *sdkClient) ListClustersInProject(project string) ([]*container.Cluster, error) {
	var clusters []*container.Cluster
	projectFullPath := fmt.Sprintf("projects/%s/locations/-", project)
	resp, err := gsc.Projects.Locations.Clusters.List(projectFullPath).Do()
	if err != nil {
		return clusters, fmt.Errorf("failed to list clusters under project %s: %v", project, err)
	}
	return resp.Clusters, nil
}

// GetOperation gets the operation ref with the given operation name.
func (gsc *sdkClient) GetOperation(project, region, zone, opName string) (*container.Operation, error) {
	location := GetClusterLocation(region, zone)
	if zone != "" {
		return gsc.Service.Projects.Zones.Operations.Get(project, location, opName).Do()
	}
	opsFullPath := fmt.Sprintf("projects/%s/locations/%s/operations/%s", project, location, opName)
	return gsc.Service.Projects.Locations.Operations.Get(opsFullPath).Do()
}
