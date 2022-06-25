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
	"time"

	"github.com/pkg/errors"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	spireconfig "github.com/tektoncd/pipeline/pkg/spire/config"
)

type spireEntrypointerAPIClient struct {
	config spireconfig.SpireConfig
	client *workloadapi.Client
}

func (w *spireEntrypointerAPIClient) setupClient(ctx context.Context) error {
	if w.client == nil {
		return w.dial(ctx)
	}
	return nil
}

func (w *spireEntrypointerAPIClient) dial(ctx context.Context) error {
	client, err := workloadapi.New(ctx, workloadapi.WithAddr("unix://"+w.config.SocketPath))
	if err != nil {
		return errors.Wrap(err, "spire workload API not initialized due to error")
	}
	w.client = client
	return nil
}

func (w *spireEntrypointerAPIClient) getxsvid(ctx context.Context) (*x509svid.SVID, error) {
	backOff := 2
	var xsvid *x509svid.SVID
	var err error
	for i := 0; i < 20; i += backOff {
		xsvid, err = w.client.FetchX509SVID(ctx)
		if err == nil {
			break
		}
		time.Sleep(time.Duration(backOff) * time.Second)
	}
	if xsvid != nil && len(xsvid.Certificates) > 0 {
		return xsvid, nil
	}
	return nil, errors.Wrap(err, "requested SVID failed to get fetched and timed out")
}

// NewSpireEntrypointerAPIClient creates a new spireEntrypointerApiClient for the entrypointer
func NewSpireEntrypointerAPIClient(c spireconfig.SpireConfig) EntrypointerAPIClient {
	return &spireEntrypointerAPIClient{
		config: c,
	}
}

func (w *spireEntrypointerAPIClient) Close() error {
	err := w.client.Close()
	if err != nil {
		return err
	}
	return nil
}
