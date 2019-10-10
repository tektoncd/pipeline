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

package clustermanager

import (
	"errors"
	"fmt"
	"log"
	"strings"

	container "google.golang.org/api/container/v1beta1"
	"knative.dev/pkg/testutils/clustermanager/boskos"
	"knative.dev/pkg/testutils/common"
	"knative.dev/pkg/testutils/gke"
)

const (
	DefaultGKEMinNodes = 1
	DefaultGKEMaxNodes = 3
	DefaultGKENodeType = "n1-standard-4"
	DefaultGKERegion   = "us-central1"
	DefaultGKEZone     = ""
	regionEnv          = "E2E_CLUSTER_REGION"
	backupRegionEnv    = "E2E_CLUSTER_BACKUP_REGIONS"
	defaultGKEVersion  = "latest"
)

var (
	DefaultGKEBackupRegions = []string{"us-west1", "us-east1"}
	protectedProjects       = []string{"knative-tests"}
	protectedClusters       = []string{"knative-prow"}
)

// GKEClient implements Client
type GKEClient struct {
}

// GKERequest contains all requests collected for cluster creation
type GKERequest struct {
	// Request holds settings for GKE native operations
	gke.Request

	// BackupRegions: fall back regions to try out in case of cluster creation
	// failure due to regional issue(s)
	BackupRegions []string

	// SkipCreation: skips cluster creation
	SkipCreation bool

	// NeedsCleanup: enforce clean up if given this option, used when running
	// locally
	NeedsCleanup bool
}

// GKECluster implements ClusterOperations
type GKECluster struct {
	Request *GKERequest
	// Project might be GKE specific, so put it here
	Project *string
	// NeedsCleanup tells whether the cluster needs to be deleted afterwards
	// This probably should be part of task wrapper's logic
	NeedsCleanup bool
	Cluster      *container.Cluster
	operations   gke.SDKOperations
	boskosOps    boskos.Operation
}

// Setup sets up a GKECluster client, takes GEKRequest as parameter and applies
// all defaults if not defined.
func (gs *GKEClient) Setup(r GKERequest) ClusterOperations {
	gc := &GKECluster{}

	if r.Project != "" { // use provided project and create cluster
		gc.Project = &r.Project
		gc.NeedsCleanup = true
	}

	if r.MinNodes == 0 {
		r.MinNodes = DefaultGKEMinNodes
	}
	if r.MaxNodes == 0 {
		r.MaxNodes = DefaultGKEMaxNodes
		// We don't want MaxNodes < MinNodes
		if r.MinNodes > r.MaxNodes {
			r.MaxNodes = r.MinNodes
		}
	}
	if r.NodeType == "" {
		r.NodeType = DefaultGKENodeType
	}
	// Only use default backup regions if region is not provided
	if len(r.BackupRegions) == 0 && r.Region == "" {
		r.BackupRegions = DefaultGKEBackupRegions
		if common.GetOSEnv(backupRegionEnv) != "" {
			r.BackupRegions = strings.Split(common.GetOSEnv(backupRegionEnv), " ")
		}
	}
	if r.Region == "" {
		r.Region = DefaultGKERegion
		if common.GetOSEnv(regionEnv) != "" {
			r.Region = common.GetOSEnv(regionEnv)
		}
	}
	if r.Zone == "" {
		r.Zone = DefaultGKEZone
	} else { // No backupregions if zone is provided
		r.BackupRegions = make([]string, 0)
	}

	gc.Request = &r

	client, err := gke.NewSDKClient()
	if err != nil {
		log.Fatalf("failed to create GKE SDK client: '%v'", err)
	}
	gc.operations = client

	gc.boskosOps = &boskos.Client{}

	return gc
}

// initialize checks environment for cluster and projects to decide whether using
// existing cluster/project or creating new ones.
func (gc *GKECluster) initialize() error {
	// Try obtain project name via `kubectl`, `gcloud`
	if gc.Project == nil {
		if err := gc.checkEnvironment(); err != nil {
			return fmt.Errorf("failed checking existing cluster: '%v'", err)
		} else if gc.Cluster != nil { // Return if Cluster was already set by kubeconfig
			// If clustername provided and kubeconfig set, ignore kubeconfig
			if gc.Request != nil && gc.Request.ClusterName != "" && gc.Cluster.Name != gc.Request.ClusterName {
				gc.Cluster = nil
			}
			if gc.Cluster != nil {
				return nil
			}
		}
	}
	// Get project name from boskos if running in Prow
	if gc.Project == nil && common.IsProw() {
		project, err := gc.boskosOps.AcquireGKEProject(nil)
		if err != nil {
			return fmt.Errorf("failed acquiring boskos project: '%v'", err)
		}
		gc.Project = &project.Name
	}
	if gc.Project == nil || *gc.Project == "" {
		return errors.New("gcp project must be set")
	}
	if !common.IsProw() && gc.Cluster == nil {
		gc.NeedsCleanup = true
	}
	log.Printf("Using project %q for running test", *gc.Project)
	return nil
}

// Provider returns gke
func (gc *GKECluster) Provider() string {
	return "gke"
}

// Acquire gets existing cluster or create a new one, the creation logic
// contains retries in BackupRegions. Default creating cluster
// in us-central1, and default BackupRegions are us-west1 and us-east1. If
// Region or Zone is provided then there is no retries
func (gc *GKECluster) Acquire() error {
	if err := gc.initialize(); err != nil {
		return fmt.Errorf("failed initializing with environment: '%v'", err)
	}
	gc.ensureProtected()
	var err error
	// Check if using existing cluster
	if gc.Cluster != nil {
		return nil
	}
	if gc.Request.SkipCreation {
		log.Println("Skipping cluster creation as SkipCreation is set")
		return nil
	}

	// Make a deep copy of the request struct, since the original request is supposed to be immutable
	request := gc.Request.DeepCopy()
	// Perform GKE specific cluster creation logics
	if request.ClusterName == "" {
		request.ClusterName, err = getResourceName(ClusterResource)
		if err != nil {
			return fmt.Errorf("failed getting cluster name: '%v'", err)
		}
	}
	if request.Project == "" {
		request.Project = *gc.Project
	}

	regions := []string{request.Region}
	for _, br := range gc.Request.BackupRegions {
		exist := false
		for _, region := range regions {
			if br == region {
				exist = true
			}
		}
		if !exist {
			regions = append(regions, br)
		}
	}
	var cluster *container.Cluster
	rb, err := gke.NewCreateClusterRequest(request)
	if err != nil {
		return fmt.Errorf("failed building the CreateClusterRequest: '%v'", err)
	}
	for i, region := range regions {
		// Restore innocence
		err = nil

		clusterName := request.ClusterName
		// Deleting cluster if it already exists
		existingCluster, _ := gc.operations.GetCluster(*gc.Project, region, request.Zone, clusterName)
		if existingCluster != nil {
			log.Printf("Cluster %q already exists in region %q zone %q. Deleting...", clusterName, region, request.Zone)
			err = gc.operations.DeleteCluster(*gc.Project, region, request.Zone, clusterName)
		}
		// Creating cluster only if previous step succeeded
		if err == nil {
			log.Printf("Creating cluster %q in region %q zone %q with:\n%+v", clusterName, region, request.Zone, gc.Request)
			err = gc.operations.CreateCluster(*gc.Project, region, request.Zone, rb)
			if err == nil { // Get cluster at last
				cluster, err = gc.operations.GetCluster(*gc.Project, region, request.Zone, rb.Cluster.Name)
			}
		}
		if err != nil {
			errMsg := fmt.Sprintf("Error during cluster creation: '%v'. ", err)
			if gc.NeedsCleanup { // Delete half created cluster if it's user created
				errMsg = fmt.Sprintf("%sDeleting cluster %q in region %q zone %q in background...\n", errMsg, clusterName, region, request.Zone)
				gc.operations.DeleteClusterAsync(*gc.Project, region, request.Zone, clusterName)
			}
			// Retry another region if cluster creation failed.
			// TODO(chaodaiG): catch specific errors as we know what the error look like for stockout etc.
			if i != len(regions)-1 {
				errMsg = fmt.Sprintf("%sRetry another region %q for cluster creation", errMsg, regions[i+1])
			}
			log.Printf(errMsg)
		} else {
			log.Print("Cluster creation completed")
			gc.Cluster = cluster
			break
		}
	}

	return err
}

// Delete takes care of GKE cluster resource cleanup. It only release Boskos resource if running in
// Prow, otherwise deletes the cluster if marked NeedsCleanup
func (gc *GKECluster) Delete() error {
	if err := gc.initialize(); err != nil {
		return fmt.Errorf("failed initializing with environment: '%v'", err)
	}
	gc.ensureProtected()
	// Release Boskos if running in Prow, will let Janitor taking care of
	// clusters deleting
	if common.IsProw() {
		log.Printf("Releasing Boskos resource: '%v'", *gc.Project)
		return gc.boskosOps.ReleaseGKEProject(nil, *gc.Project)
	}

	// NeedsCleanup is only true if running locally and cluster created by the
	// process
	if !gc.NeedsCleanup && !gc.Request.NeedsCleanup {
		return nil
	}
	// Should only get here if running locally and cluster created by this
	// client, so at this moment cluster should have been set
	if gc.Cluster == nil {
		return fmt.Errorf("cluster doesn't exist")
	}
	log.Printf("Deleting cluster %q in %q", gc.Cluster.Name, gc.Cluster.Location)
	region, zone := gke.RegionZoneFromLoc(gc.Cluster.Location)
	if err := gc.operations.DeleteCluster(*gc.Project, region, zone, gc.Cluster.Name); err != nil {
		return fmt.Errorf("failed deleting cluster: '%v'", err)
	}
	return nil
}

// ensureProtected ensures not operating on protected project/cluster
func (gc *GKECluster) ensureProtected() {
	if gc.Project != nil {
		for _, pp := range protectedProjects {
			if *gc.Project == pp {
				log.Fatalf("project %q is protected", *gc.Project)
			}
		}
	}
	if gc.Cluster != nil {
		for _, pc := range protectedClusters {
			if gc.Cluster.Name == pc {
				log.Fatalf("cluster %q is protected", gc.Cluster.Name)
			}
		}
	}
}

// checks for existing cluster by looking at kubeconfig,
// and sets up gc.Project and gc.Cluster properly, otherwise fail it.
// if project can be derived from gcloud, sets it up as well
func (gc *GKECluster) checkEnvironment() error {
	var err error
	// if kubeconfig is configured, use it
	output, err := common.StandardExec("kubectl", "config", "current-context")
	if err == nil {
		currentContext := strings.TrimSpace(string(output))
		if strings.HasPrefix(currentContext, "gke_") {
			// output should be in the form of gke_PROJECT_REGION_CLUSTER
			parts := strings.Split(currentContext, "_")
			if len(parts) != 4 { // fall through with warning
				log.Printf("WARNING: ignoring kubectl current-context since it's malformed: '%s'", currentContext)
			} else {
				log.Printf("kubeconfig isn't empty, uses this cluster for running tests: %s", currentContext)
				gc.Project = &parts[1]
				location, clusterName := parts[2], parts[3]
				region, zone := gke.RegionZoneFromLoc(location)
				gc.Cluster, err = gc.operations.GetCluster(*gc.Project, region, zone, clusterName)
				if err != nil {
					return fmt.Errorf("couldn't find cluster %s in %s in %s, does it exist? %v", clusterName, *gc.Project, location, err)
				}
				return nil
			}
		}
	}
	if err != nil && len(output) > 0 {
		// this is unexpected error, should shout out directly
		return fmt.Errorf("failed running kubectl config current-context: '%s'", string(output))
	}

	// if gcloud is pointing to a project, use it
	output, err = common.StandardExec("gcloud", "config", "get-value", "project")
	if err != nil {
		return fmt.Errorf("failed getting gcloud project: '%v'", err)
	}
	if string(output) != "" {
		project := strings.Trim(strings.TrimSpace(string(output)), "\n\r")
		gc.Project = &project
	}

	return nil
}
