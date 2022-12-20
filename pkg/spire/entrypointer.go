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

// NewEntrypointerAPIClient creates the EntrypointerAPIClient
func NewEntrypointerAPIClient(c *spireconfig.SpireConfig) EntrypointerAPIClient {
	return &spireEntrypointerAPIClient{
		config: c,
	}
}

type spireEntrypointerAPIClient struct {
	config *spireconfig.SpireConfig
	client *workloadapi.Client
}

func (w *spireEntrypointerAPIClient) setupClient(ctx context.Context) error {
	if w.config == nil {
		return errors.New("config has not been set yet")
	}
	if w.client == nil {
		return w.dial(ctx)
	}
	return nil
}

func (w *spireEntrypointerAPIClient) dial(ctx context.Context) error {
	// spire workloadapi client for entrypoint - https://github.com/spiffe/go-spiffe/blob/main/v2/workloadapi/client.go
	client, err := workloadapi.New(ctx, workloadapi.WithAddr(w.config.SocketPath))
	if err != nil {
		return errors.Wrap(err, "spire workload API not initialized due to error")
	}
	w.client = client
	return nil
}

// package-level timeout and backoff enable shortened timeout for unit tests
var (
	timeout = 20 * time.Second
	backoff = 2 * time.Second
)

func (w *spireEntrypointerAPIClient) getWorkloadSVID(ctx context.Context) (*x509svid.SVID, error) {
	// Should this be using exponential backoff? IDK enough about the underlying
	// implementation to know if exponential backoff is in fact justified, so
	// when I modified this code to use a ticker I didn't change the backoff
	// logic.
	ticker := time.NewTicker(backoff)
	defer ticker.Stop()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		var xsvid *x509svid.SVID
		var err error
		if xsvid, err = w.client.FetchX509SVID(ctx); err == nil { // No err -- return immediately on success
			return xsvid, nil
		}
		select {
		case <-ticker.C:
			// do nothing; loop will try again
		case <-ctx.Done():
			// ctx timed out or was cancelled
			return nil, errors.Wrap(ctx.Err(), errors.Wrap(err, "failed to fetch SVID").Error())
		}
	}
}

func (w *spireEntrypointerAPIClient) Close() error {
	if w.client != nil {
		return w.client.Close()
	}
	return nil
}
