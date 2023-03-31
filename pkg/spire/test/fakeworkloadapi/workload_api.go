/*
Copyright 2023 The Tekton Authors

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

package fakeworkloadapi

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/spiffe/go-spiffe/v2/bundle/jwtbundle"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tektoncd/pipeline/pkg/spire/test/pemutil"
	"github.com/tektoncd/pipeline/pkg/spire/test/x509util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

var noIdentityError = status.Error(codes.PermissionDenied, "no identity issued")

type WorkloadAPI struct {
	tb               testing.TB
	wg               sync.WaitGroup
	addr             string
	server           *grpc.Server
	mu               sync.Mutex
	x509Resp         *workload.X509SVIDResponse
	x509Chans        map[chan *workload.X509SVIDResponse]struct{}
	jwtResp          *workload.JWTSVIDResponse
	jwtBundlesResp   *workload.JWTBundlesResponse
	jwtBundlesChans  map[chan *workload.JWTBundlesResponse]struct{}
	x509BundlesResp  *workload.X509BundlesResponse
	x509BundlesChans map[chan *workload.X509BundlesResponse]struct{}
}

func New(tb testing.TB) *WorkloadAPI {
	w := &WorkloadAPI{
		x509Chans:        make(map[chan *workload.X509SVIDResponse]struct{}),
		jwtBundlesChans:  make(map[chan *workload.JWTBundlesResponse]struct{}),
		x509BundlesChans: make(map[chan *workload.X509BundlesResponse]struct{}),
	}

	listener, err := newListener()
	require.NoError(tb, err)

	server := grpc.NewServer()
	workload.RegisterSpiffeWorkloadAPIServer(server, &workloadAPIWrapper{w: w})

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		_ = server.Serve(listener)
	}()

	w.addr = getTargetName(listener.Addr())
	tb.Logf("WorkloadAPI address: %s", w.addr)
	w.server = server
	return w
}

func (w *WorkloadAPI) Stop() {
	w.server.Stop()
	w.wg.Wait()
}

func (w *WorkloadAPI) Addr() string {
	return w.addr
}

func (w *WorkloadAPI) SetX509SVIDResponse(r *X509SVIDResponse) {
	var resp *workload.X509SVIDResponse
	if r != nil {
		resp = r.ToProto(w.tb)
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.x509Resp = resp

	for ch := range w.x509Chans {
		select {
		case ch <- resp:
		default:
			<-ch
			ch <- resp
		}
	}
}

func (w *WorkloadAPI) SetJWTSVIDResponse(r *workload.JWTSVIDResponse) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if r != nil {
		w.jwtResp = r
	}
}

func (w *WorkloadAPI) SetJWTBundles(jwtBundles ...*jwtbundle.Bundle) {
	resp := &workload.JWTBundlesResponse{
		Bundles: make(map[string][]byte),
	}
	for _, bundle := range jwtBundles {
		bundleBytes, err := bundle.Marshal()
		assert.NoError(w.tb, err)
		resp.Bundles[bundle.TrustDomain().String()] = bundleBytes
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.jwtBundlesResp = resp

	for ch := range w.jwtBundlesChans {
		select {
		case ch <- w.jwtBundlesResp:
		default:
			<-ch
			ch <- w.jwtBundlesResp
		}
	}
}

func (w *WorkloadAPI) SetX509Bundles(x509Bundles ...*x509bundle.Bundle) {
	resp := &workload.X509BundlesResponse{
		Bundles: make(map[string][]byte),
	}
	for _, bundle := range x509Bundles {
		bundleBytes, err := bundle.Marshal()
		assert.NoError(w.tb, err)
		bundlePem, err := pemutil.ParseCertificates(bundleBytes)
		assert.NoError(w.tb, err)

		var rawBytes []byte
		for _, c := range bundlePem {
			rawBytes = append(rawBytes, c.Raw...)
		}

		resp.Bundles[bundle.TrustDomain().String()] = rawBytes
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.x509BundlesResp = resp

	for ch := range w.x509BundlesChans {
		select {
		case ch <- w.x509BundlesResp:
		default:
			<-ch
			ch <- w.x509BundlesResp
		}
	}
}

type workloadAPIWrapper struct {
	workload.UnimplementedSpiffeWorkloadAPIServer
	w *WorkloadAPI
}

func (w *workloadAPIWrapper) FetchX509SVID(req *workload.X509SVIDRequest, stream workload.SpiffeWorkloadAPI_FetchX509SVIDServer) error {
	return w.w.fetchX509SVID(req, stream)
}

func (w *workloadAPIWrapper) FetchX509Bundles(req *workload.X509BundlesRequest, stream workload.SpiffeWorkloadAPI_FetchX509BundlesServer) error {
	return w.w.fetchX509Bundles(req, stream)
}

func (w *workloadAPIWrapper) FetchJWTSVID(ctx context.Context, req *workload.JWTSVIDRequest) (*workload.JWTSVIDResponse, error) {
	return w.w.fetchJWTSVID(ctx, req)
}

func (w *workloadAPIWrapper) FetchJWTBundles(req *workload.JWTBundlesRequest, stream workload.SpiffeWorkloadAPI_FetchJWTBundlesServer) error {
	return w.w.fetchJWTBundles(req, stream)
}

func (w *workloadAPIWrapper) ValidateJWTSVID(ctx context.Context, req *workload.ValidateJWTSVIDRequest) (*workload.ValidateJWTSVIDResponse, error) {
	return w.w.validateJWTSVID(ctx, req)
}

type X509SVIDResponse struct {
	SVIDs            []*x509svid.SVID
	Bundle           *x509bundle.Bundle
	FederatedBundles []*x509bundle.Bundle
}

func (r *X509SVIDResponse) ToProto(tb testing.TB) *workload.X509SVIDResponse {
	var bundle []byte
	if r.Bundle != nil {
		bundle = x509util.ConcatRawCertsFromCerts(r.Bundle.X509Authorities())
	}

	pb := &workload.X509SVIDResponse{
		FederatedBundles: make(map[string][]byte),
	}
	for _, svid := range r.SVIDs {
		var keyDER []byte
		if svid.PrivateKey != nil {
			var err error
			keyDER, err = x509.MarshalPKCS8PrivateKey(svid.PrivateKey)
			require.NoError(tb, err)
		}
		pb.Svids = append(pb.Svids, &workload.X509SVID{
			SpiffeId:    svid.ID.String(),
			X509Svid:    x509util.ConcatRawCertsFromCerts(svid.Certificates),
			X509SvidKey: keyDER,
			Bundle:      bundle,
		})
	}
	for _, v := range r.FederatedBundles {
		pb.FederatedBundles[v.TrustDomain().IDString()] = x509util.ConcatRawCertsFromCerts(v.X509Authorities())
	}

	return pb
}

func (w *WorkloadAPI) fetchX509SVID(_ *workload.X509SVIDRequest, stream workload.SpiffeWorkloadAPI_FetchX509SVIDServer) error {
	if err := checkHeader(stream.Context()); err != nil {
		return err
	}
	ch := make(chan *workload.X509SVIDResponse, 1)
	w.mu.Lock()
	w.x509Chans[ch] = struct{}{}
	resp := w.x509Resp
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		delete(w.x509Chans, ch)
		w.mu.Unlock()
	}()

	sendResp := func(resp *workload.X509SVIDResponse) error {
		if resp == nil {
			return noIdentityError
		}
		return stream.Send(resp)
	}

	if err := sendResp(resp); err != nil {
		return err
	}
	for {
		select {
		case resp := <-ch:
			if err := sendResp(resp); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

func (w *WorkloadAPI) fetchX509Bundles(_ *workload.X509BundlesRequest, stream workload.SpiffeWorkloadAPI_FetchX509BundlesServer) error {
	if err := checkHeader(stream.Context()); err != nil {
		return err
	}
	ch := make(chan *workload.X509BundlesResponse, 1)
	w.mu.Lock()
	w.x509BundlesChans[ch] = struct{}{}
	resp := w.x509BundlesResp
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		delete(w.x509BundlesChans, ch)
		w.mu.Unlock()
	}()

	sendResp := func(resp *workload.X509BundlesResponse) error {
		if resp == nil {
			return noIdentityError
		}
		return stream.Send(resp)
	}

	if err := sendResp(resp); err != nil {
		return err
	}
	for {
		select {
		case resp := <-ch:
			if err := sendResp(resp); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

func (w *WorkloadAPI) fetchJWTSVID(ctx context.Context, req *workload.JWTSVIDRequest) (*workload.JWTSVIDResponse, error) {
	if err := checkHeader(ctx); err != nil {
		return nil, err
	}
	if len(req.Audience) == 0 {
		return nil, errors.New("no audience")
	}
	if w.jwtResp == nil {
		return nil, noIdentityError
	}

	return w.jwtResp, nil
}

func (w *WorkloadAPI) fetchJWTBundles(_ *workload.JWTBundlesRequest, stream workload.SpiffeWorkloadAPI_FetchJWTBundlesServer) error {
	if err := checkHeader(stream.Context()); err != nil {
		return err
	}
	ch := make(chan *workload.JWTBundlesResponse, 1)
	w.mu.Lock()
	w.jwtBundlesChans[ch] = struct{}{}
	resp := w.jwtBundlesResp
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		delete(w.jwtBundlesChans, ch)
		w.mu.Unlock()
	}()

	sendResp := func(resp *workload.JWTBundlesResponse) error {
		if resp == nil {
			return noIdentityError
		}
		return stream.Send(resp)
	}

	if err := sendResp(resp); err != nil {
		return err
	}
	for {
		select {
		case resp := <-ch:
			if err := sendResp(resp); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

func (w *WorkloadAPI) validateJWTSVID(_ context.Context, req *workload.ValidateJWTSVIDRequest) (*workload.ValidateJWTSVIDResponse, error) {
	if req.Audience == "" {
		return nil, status.Error(codes.InvalidArgument, "audience must be specified")
	}

	if req.Svid == "" {
		return nil, status.Error(codes.InvalidArgument, "svid must be specified")
	}

	// TODO: validate
	jwtSvid, err := jwtsvid.ParseInsecure(req.Svid, []string{req.Audience})
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	claims, err := structFromValues(jwtSvid.Claims)
	require.NoError(w.tb, err)

	return &workload.ValidateJWTSVIDResponse{
		SpiffeId: jwtSvid.ID.String(),
		Claims:   claims,
	}, nil
}

func checkHeader(ctx context.Context) error {
	return checkMetadata(ctx, "workload.spiffe.io", "true")
}

func checkMetadata(ctx context.Context, key, value string) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errors.New("request does not contain metadata")
	}
	values := md.Get(key)
	if len(value) == 0 {
		return fmt.Errorf("request metadata does not contain %q value", key)
	}
	if values[0] != value {
		return fmt.Errorf("request metadata %q value is %q; expected %q", key, values[0], value)
	}
	return nil
}

func structFromValues(values map[string]interface{}) (*structpb.Struct, error) {
	valuesJSON, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	s := new(structpb.Struct)
	if err := protojson.Unmarshal(valuesJSON, s); err != nil {
		return nil, err
	}

	return s, nil
}
