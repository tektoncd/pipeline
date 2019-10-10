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

package mako

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"

	"cloud.google.com/go/compute/metadata"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	"github.com/google/mako/go/quickstore"
	qpb "github.com/google/mako/proto/quickstore/quickstore_go_proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/changeset"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
)

const (
	// sidecarAddress is the address of the Mako sidecar to which we locally
	// write results, and it authenticates and publishes them to Mako after
	// assorted preprocessing.
	sidecarAddress = "localhost:9813"
)

// EscapeTag replaces characters that Mako doesn't accept with ones it does.
func EscapeTag(tag string) string {
	return strings.ReplaceAll(tag, ".", "_")
}

// Setup sets up the mako client for the provided benchmarkKey.
// It will add a few common tags and allow each benchmark to add custm tags as well.
// It returns the mako client handle to store metrics, a method to close the connection
// to mako server once done and error in case of failures.
func Setup(ctx context.Context, extraTags ...string) (context.Context, *quickstore.Quickstore, func(context.Context), error) {
	tags := append(MustGetTags(), extraTags...)
	// Get the commit of the benchmarks
	commitID, err := changeset.Get()
	if err != nil {
		return nil, nil, nil, err
	}

	// Setup a deployment informer, so that we can use the lister to track
	// desired and available pod counts.
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, informers := injection.Default.SetupInformers(ctx, cfg)
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		return nil, nil, nil, err
	}

	// Get the Kubernetes version from the API server.
	kc := kubeclient.Get(ctx)
	version, err := kc.Discovery().ServerVersion()
	if err != nil {
		return nil, nil, nil, err
	}

	// Determine the number of Kubernetes nodes through the kubernetes client.
	nodes, err := kc.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, nil, nil, err
	}
	tags = append(tags, "nodes="+fmt.Sprintf("%d", len(nodes.Items)))

	// Decorate GCP metadata as tags (when we're running on GCP).
	if projectID, err := metadata.ProjectID(); err != nil {
		log.Printf("GCP project ID is not available: %v", err)
	} else {
		tags = append(tags, "project-id="+EscapeTag(projectID))
	}
	if zone, err := metadata.Zone(); err != nil {
		log.Printf("GCP zone is not available: %v", err)
	} else {
		tags = append(tags, "zone="+EscapeTag(zone))
	}
	if machineType, err := metadata.Get("instance/machine-type"); err != nil {
		log.Printf("GCP machine type is not available: %v", err)
	} else if parts := strings.Split(machineType, "/"); len(parts) != 4 {
		tags = append(tags, "instanceType="+EscapeTag(parts[3]))
	}

	qs, qclose, err := quickstore.NewAtAddress(ctx, &qpb.QuickstoreInput{
		BenchmarkKey: MustGetBenchmark(),
		Tags: append(tags,
			"commit="+commitID,
			"kubernetes="+EscapeTag(version.String()),
			EscapeTag(runtime.Version()),
		),
	}, sidecarAddress)
	if err != nil {
		return nil, nil, nil, err
	}
	return ctx, qs, qclose, nil
}
