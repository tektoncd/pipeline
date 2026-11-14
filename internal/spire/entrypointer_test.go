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
	"errors"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/spire/config"
	"github.com/tektoncd/pipeline/pkg/spire/test/fakeworkloadapi"
)

func init() {
	// shorten timeout and backoff intervals for faster tests;
	// variables are declared in entrypointer.go
	timeout = time.Second
	backoff = 100 * time.Millisecond
}

func TestNoSVID(t *testing.T) {
	ctx := t.Context()
	wl := fakeworkloadapi.New(t)
	defer wl.Stop()
	cfg := &config.SpireConfig{SocketPath: wl.Addr()}
	ec := NewEntrypointerAPIClient(cfg)
	defer ec.Close()

	if _, err := ec.Sign(ctx, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("got %v; want %v", err, context.DeadlineExceeded)
	}
}
