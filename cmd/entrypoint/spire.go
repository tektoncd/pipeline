//go:build !disable_spire

/*
Copyright 2025 The Tekton Authors

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

package main

import (
	"flag"
	"log"

	"github.com/tektoncd/pipeline/pkg/spire"
	"github.com/tektoncd/pipeline/pkg/spire/config"
)

var (
	enableSpire = flag.Bool("enable_spire", false, "If specified by configmap, this enables spire signing and verification")
	socketPath  = flag.String("spire_socket_path", "unix:///spiffe-workload-api/spire-agent.sock", "Experimental: The SPIRE agent socket for SPIFFE workload API.")
)

func initializeSpireAPI() spire.EntrypointerAPIClient {
	if enableSpire != nil && *enableSpire && socketPath != nil && *socketPath != "" {
		log.Println("SPIRE is enabled in this build, enableSpire is supported")
		spireConfig := config.SpireConfig{
			SocketPath: *socketPath,
		}
		return spire.NewEntrypointerAPIClient(&spireConfig)
	}
	return nil
}
