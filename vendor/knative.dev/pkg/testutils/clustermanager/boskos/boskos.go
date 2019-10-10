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

package boskos

import (
	"context"
	"fmt"
	"time"

	"knative.dev/pkg/testutils/common"

	boskosclient "k8s.io/test-infra/boskos/client"
	boskoscommon "k8s.io/test-infra/boskos/common"
)

const (
	// GKEProjectResource is resource type defined for GKE projects
	GKEProjectResource = "gke-project"
)

var (
	boskosURI           = "http://boskos.test-pods.svc.cluster.local."
	defaultWaitDuration = time.Minute * 20
)

func newClient(host *string) *boskosclient.Client {
	if nil == host {
		hostName := common.GetOSEnv("JOB_NAME")
		host = &hostName
	}
	return boskosclient.NewClient(*host, boskosURI)
}

// AcquireGKEProject acquires GKE Boskos Project with "free" state, and not
// owned by anyone, sets its state to "busy" and assign it an owner of *host,
// which by default is env var `JOB_NAME`.
func AcquireGKEProject(host *string) (*boskoscommon.Resource, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultWaitDuration)
	defer cancel()
	p, err := newClient(host).AcquireWait(ctx, GKEProjectResource, boskoscommon.Free, boskoscommon.Busy)
	if nil != err {
		return nil, fmt.Errorf("boskos failed to acquire GKE project: %v", err)
	}
	if p == nil {
		return nil, fmt.Errorf("boskos does not have a free %s at the moment", GKEProjectResource)
	}
	return p, nil
}

// ReleaseGKEProject releases project, the host must match with the host name that acquired
// the project, which by default is env var `JOB_NAME`. The state is set to
// "dirty" for Janitor picking up.
// This function is very powerful, it can release Boskos resource acquired by
// other processes, regardless of where the other process is running.
func ReleaseGKEProject(host *string, name string) error {
	client := newClient(host)
	if err := client.Release(name, boskoscommon.Dirty); nil != err {
		return fmt.Errorf("boskos failed to release GKE project '%s': %v", name, err)
	}
	return nil
}
