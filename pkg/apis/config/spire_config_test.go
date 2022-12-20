/*
Copyright 2021 The Tekton Authors

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

package config_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	test "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	sc "github.com/tektoncd/pipeline/pkg/spire/config"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestNewSpireConfigFromConfigMap(t *testing.T) {
	for _, tc := range []struct {
		want     *sc.SpireConfig
		fileName string
	}{
		{
			want: &sc.SpireConfig{
				TrustDomain:     "test.com",
				SocketPath:      "unix:///test-spire-api/test-spire-agent.sock",
				ServerAddr:      "test-spire-server.spire.svc.cluster.local:8081",
				NodeAliasPrefix: "/test-tekton-node/",
			},
			fileName: config.GetSpireConfigName(),
		},
		{
			want: &sc.SpireConfig{
				TrustDomain:     "example.org",
				SocketPath:      "unix:///spiffe-workload-api/spire-agent.sock",
				ServerAddr:      "spire-server.spire.svc.cluster.local:8081",
				NodeAliasPrefix: "/tekton-node/",
			},
			fileName: "config-spire-empty",
		},
	} {
		cm := test.ConfigMapFromTestFile(t, tc.fileName)
		if got, err := config.NewSpireConfigFromConfigMap(cm); err == nil {
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("Diff:\n%s", diff.PrintWantGot(d))
			}
		} else {
			t.Errorf("NewSpireConfigFromConfigMap(actual) = %v", err)
		}
	}
}
