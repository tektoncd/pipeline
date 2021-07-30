/*
Copyright 2018 The Knative Authors

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

// This file contains logic to encapsulate flags which are needed to specify
// what cluster, etc. to use for e2e tests.

package test

import (
	"bytes"
	"flag"
	"text/template"

	env "knative.dev/pkg/environment"
	testenv "knative.dev/pkg/test/environment"
	"knative.dev/pkg/test/logging"
)

var (
	// Flags holds the command line flags or defaults for settings in the user's environment.
	// See EnvironmentFlags for a list of supported fields.
	// Deprecated: use test/flags.Flags()
	Flags = initializeFlags()
)

// EnvironmentFlags define the flags that are needed to run the e2e tests.
// Deprecated: use test/flags.Flags() or injection.Flags()
type EnvironmentFlags struct {
	env.ClientConfig
	testenv.TestClientConfig
}

func initializeFlags() *EnvironmentFlags {
	f := new(EnvironmentFlags)

	f.ClientConfig.InitFlags(flag.CommandLine)
	f.TestClientConfig.InitFlags(flag.CommandLine)

	return f
}

// SetupLoggingFlags initializes a logger for tests.
// TODO(coryrc): Remove once other repos are moved to call logging.InitializeLogger() directly
func SetupLoggingFlags() {
	logging.InitializeLogger()
}

// ImagePath is a helper function to transform an image name into an image reference that can be pulled.
func ImagePath(name string) string {
	tpl, err := template.New("image").Parse(Flags.ImageTemplate)
	if err != nil {
		panic("could not parse image template: " + err.Error())
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, struct {
		Repository string
		Name       string
		Tag        string
	}{
		Repository: Flags.DockerRepo,
		Name:       name,
		Tag:        Flags.Tag,
	}); err != nil {
		panic("could not apply the image template: " + err.Error())
	}
	return buf.String()
}
