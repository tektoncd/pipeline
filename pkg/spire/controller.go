/*
Copyright 2022 The Tekton Authors

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

package spire

import (
	"context"
	"fmt"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	entryv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/entry/v1"
	spiffetypes "github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	spireconfig "github.com/tektoncd/pipeline/pkg/spire/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	corev1 "k8s.io/api/core/v1"
)

type spireControllerAPIClient struct {
	config       spireconfig.SpireConfig
	serverConn   *grpc.ClientConn
	workloadConn *workloadapi.X509Source
	entryClient  entryv1.EntryClient
	workloadAPI  *workloadapi.Client
}

func (sc *spireControllerAPIClient) setupClient(ctx context.Context) error {
	if sc.entryClient == nil || sc.workloadConn == nil || sc.workloadAPI == nil || sc.serverConn == nil {
		return sc.dial(ctx)
	}
	return nil
}

func (sc *spireControllerAPIClient) dial(ctx context.Context) error {
	if sc.workloadConn == nil {
		// Create X509Source
		source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr("unix://"+sc.config.SocketPath)))
		if err != nil {
			return fmt.Errorf("unable to create X509Source for SPIFFE client: %w", err)
		}
		sc.workloadConn = source
	}

	if sc.workloadAPI == nil {
		client, err := workloadapi.New(ctx, workloadapi.WithAddr("unix://"+sc.config.SocketPath))
		if err != nil {
			return fmt.Errorf("spire workload API not initialized due to error: %w", err)
		}
		sc.workloadAPI = client
	}

	if sc.serverConn == nil {
		// Create connection
		tlsConfig := tlsconfig.MTLSClientConfig(sc.workloadConn, sc.workloadConn, tlsconfig.AuthorizeAny())
		conn, err := grpc.DialContext(ctx, sc.config.ServerAddr, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		if err != nil {
			sc.workloadConn.Close()
			sc.workloadConn = nil
			return fmt.Errorf("unable to dial SPIRE server: %w", err)
		}
		sc.serverConn = conn
	}

	if sc.entryClient == nil {
		sc.entryClient = entryv1.NewEntryClient(sc.serverConn)
	}

	return nil
}

// NewSpireControllerAPIClient creates a new NewSpireControllerAPIClient for the pipeline controller
func NewSpireControllerAPIClient(c spireconfig.SpireConfig) ControllerAPIClient {
	if c.MockSpire {
		return &MockClient{}
	}
	return &spireControllerAPIClient{
		config: c,
	}
}

func (sc *spireControllerAPIClient) nodeEntry(nodeName string) *spiffetypes.Entry {
	selectors := []*spiffetypes.Selector{
		{
			Type:  "k8s_psat",
			Value: "agent_ns:spire",
		},
		{
			Type:  "k8s_psat",
			Value: "agent_node_name:" + nodeName,
		},
	}

	return &spiffetypes.Entry{
		SpiffeId: &spiffetypes.SPIFFEID{
			TrustDomain: sc.config.TrustDomain,
			Path:        fmt.Sprintf("%v%v", sc.config.NodeAliasPrefix, nodeName),
		},
		ParentId: &spiffetypes.SPIFFEID{
			TrustDomain: sc.config.TrustDomain,
			Path:        "/spire/server",
		},
		Selectors: selectors,
	}
}

func (sc *spireControllerAPIClient) workloadEntry(tr *v1beta1.TaskRun, pod *corev1.Pod, expiry int64) *spiffetypes.Entry {
	// Note: We can potentially add attestation on the container images as well since
	// the information is available here.
	selectors := []*spiffetypes.Selector{
		{
			Type:  "k8s",
			Value: "pod-uid:" + string(pod.UID),
		},
		{
			Type:  "k8s",
			Value: "pod-name:" + pod.Name,
		},
	}

	return &spiffetypes.Entry{
		SpiffeId: &spiffetypes.SPIFFEID{
			TrustDomain: sc.config.TrustDomain,
			Path:        fmt.Sprintf("/ns/%v/taskrun/%v", tr.Namespace, tr.Name),
		},
		ParentId: &spiffetypes.SPIFFEID{
			TrustDomain: sc.config.TrustDomain,
			Path:        fmt.Sprintf("%v%v", sc.config.NodeAliasPrefix, pod.Spec.NodeName),
		},
		Selectors: selectors,
		ExpiresAt: expiry,
	}
}

// ttl is the TTL for the SPIRE entry in seconds, not the SVID TTL
func (sc *spireControllerAPIClient) CreateEntries(ctx context.Context, tr *v1beta1.TaskRun, pod *corev1.Pod, ttl int) error {
	err := sc.setupClient(ctx)
	if err != nil {
		return err
	}

	expiryTime := time.Now().Unix() + int64(ttl)
	entries := []*spiffetypes.Entry{
		sc.nodeEntry(pod.Spec.NodeName),
		sc.workloadEntry(tr, pod, expiryTime),
	}

	req := entryv1.BatchCreateEntryRequest{
		Entries: entries,
	}

	resp, err := sc.entryClient.BatchCreateEntry(ctx, &req)
	if err != nil {
		return err
	}

	if len(resp.Results) != len(entries) {
		return fmt.Errorf("batch create entry failed, malformed response expected %v result", len(entries))
	}

	var errPaths []string
	var errCodes []int32

	for _, r := range resp.Results {
		if codes.Code(r.Status.Code) != codes.AlreadyExists &&
			codes.Code(r.Status.Code) != codes.OK {
			errPaths = append(errPaths, r.Entry.SpiffeId.Path)
			errCodes = append(errCodes, r.Status.Code)
		}
	}

	if len(errPaths) != 0 {
		return fmt.Errorf("batch create entry failed for entries %+v with codes %+v", errPaths, errCodes)
	}
	return nil
}

func (sc *spireControllerAPIClient) getEntries(ctx context.Context, tr *v1beta1.TaskRun, pod *corev1.Pod) ([]*spiffetypes.Entry, error) {
	req := &entryv1.ListEntriesRequest{
		Filter: &entryv1.ListEntriesRequest_Filter{
			BySpiffeId: &spiffetypes.SPIFFEID{
				TrustDomain: sc.config.TrustDomain,
				Path:        fmt.Sprintf("/ns/%v/taskrun/%v", tr.Namespace, tr.Name),
			},
		},
	}

	entries := []*spiffetypes.Entry{}
	for {
		resp, err := sc.entryClient.ListEntries(ctx, req)
		if err != nil {
			return nil, err
		}

		entries = append(entries, resp.Entries...)

		if resp.NextPageToken == "" {
			break
		}

		req.PageToken = resp.NextPageToken
	}

	return entries, nil
}

func (sc *spireControllerAPIClient) DeleteEntry(ctx context.Context, tr *v1beta1.TaskRun, pod *corev1.Pod) error {
	entries, err := sc.getEntries(ctx, tr, pod)
	if err != nil {
		return err
	}

	var ids []string
	for _, e := range entries {
		ids = append(ids, e.Id)
	}

	req := &entryv1.BatchDeleteEntryRequest{
		Ids: ids,
	}
	resp, err := sc.entryClient.BatchDeleteEntry(ctx, req)
	if err != nil {
		return err
	}

	var errIds []string
	var errCodes []int32

	for _, r := range resp.Results {
		if codes.Code(r.Status.Code) != codes.NotFound &&
			codes.Code(r.Status.Code) != codes.OK {
			errIds = append(errIds, r.Id)
			errCodes = append(errCodes, r.Status.Code)
		}
	}

	if len(errIds) != 0 {
		return fmt.Errorf("batch delete entry failed for ids %+v with codes %+v", errIds, errCodes)
	}

	return nil
}

func (sc *spireControllerAPIClient) Close() error {
	err := sc.serverConn.Close()
	if err != nil {
		return err
	}
	err = sc.workloadAPI.Close()
	if err != nil {
		return err
	}
	err = sc.workloadConn.Close()
	if err != nil {
		return err
	}
	return nil
}
