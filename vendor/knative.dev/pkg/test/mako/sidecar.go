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
	"path/filepath"
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
	"knative.dev/pkg/test/mako/alerter"
	"knative.dev/pkg/test/mako/config"
)

const (
	// sidecarAddress is the address of the Mako sidecar to which we locally
	// write results, and it authenticates and publishes them to Mako after
	// assorted preprocessing.
	sidecarAddress = "localhost:9813"

	// org is the orgnization name that is used by Github client
	org = "knative"

	// slackUserName is the slack user name that is used by Slack client
	slackUserName = "Knative Testgrid Robot"

	// These token settings are for alerter.
	// If we want to enable the alerter for a benchmark, we need to mount the
	// token to the pod, with the same name and path.
	// See https://github.com/knative/serving/blob/master/test/performance/dataplane-probe/dataplane-probe.yaml
	tokenFolder     = "/var/secret"
	githubToken     = "github-token"
	slackReadToken  = "slack-read-token"
	slackWriteToken = "slack-write-token"
)

// Client is a wrapper that wraps all Mako related operations
type Client struct {
	Quickstore    *quickstore.Quickstore
	Context       context.Context
	ShutDownFunc  func(context.Context)
	benchmarkName string
	alerter       *alerter.Alerter
}

// StoreAndHandleResult stores the benchmarking data and handles the result.
func (c *Client) StoreAndHandleResult() error {
	out, err := c.Quickstore.Store()
	return c.alerter.HandleBenchmarkResult(c.benchmarkName, out, err)
}

// EscapeTag replaces characters that Mako doesn't accept with ones it does.
func EscapeTag(tag string) string {
	return strings.ReplaceAll(tag, ".", "_")
}

// SetupHelper sets up the mako client for the provided benchmarkKey.
// It will add a few common tags and allow each benchmark to add custm tags as well.
// It returns the mako client handle to store metrics, a method to close the connection
// to mako server once done and error in case of failures.
func SetupHelper(ctx context.Context, benchmarkKey *string, benchmarkName *string, extraTags ...string) (*Client, error) {
	tags := append(config.MustGetTags(), extraTags...)
	// Get the commit of the benchmarks
	commitID, err := changeset.Get()
	if err != nil {
		return nil, err
	}

	// Setup a deployment informer, so that we can use the lister to track
	// desired and available pod counts.
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	ctx, informers := injection.Default.SetupInformers(ctx, cfg)
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		return nil, err
	}

	// Get the Kubernetes version from the API server.
	kc := kubeclient.Get(ctx)
	version, err := kc.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	// Determine the number of Kubernetes nodes through the kubernetes client.
	nodes, err := kc.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
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

	// Create a new Quickstore that connects to the microservice
	qs, qclose, err := quickstore.NewAtAddress(ctx, &qpb.QuickstoreInput{
		BenchmarkKey: benchmarkKey,
		Tags: append(tags,
			"commit="+commitID,
			"kubernetes="+EscapeTag(version.String()),
			EscapeTag(runtime.Version()),
		),
	}, sidecarAddress)
	if err != nil {
		return nil, err
	}

	// Create a new Alerter that alerts for performance regressions
	alerter := &alerter.Alerter{}
	alerter.SetupGitHub(
		org,
		config.GetRepository(),
		tokenPath(githubToken),
	)
	alerter.SetupSlack(
		slackUserName,
		tokenPath(slackReadToken),
		tokenPath(slackWriteToken),
		config.GetSlackChannels(*benchmarkName),
	)

	client := &Client{
		Quickstore:    qs,
		Context:       ctx,
		ShutDownFunc:  qclose,
		alerter:       alerter,
		benchmarkName: *benchmarkName,
	}

	return client, nil
}

func Setup(ctx context.Context, extraTags ...string) (*Client, error) {
	benchmarkKey, benchmarkName := config.MustGetBenchmark()
	return SetupHelper(ctx, benchmarkKey, benchmarkName, extraTags...)
}

func SetupWithBenchmarkConfig(ctx context.Context, benchmarkKey *string, benchmarkName *string, extraTags ...string) (*Client, error) {
	return SetupHelper(ctx, benchmarkKey, benchmarkName, extraTags...)
}

func tokenPath(token string) string {
	return filepath.Join(tokenFolder, token)
}
