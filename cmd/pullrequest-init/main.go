/*
Copyright 2019 The Tekton Authors

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
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/tektoncd/pipeline/pkg/pullrequest"
	"go.uber.org/zap"
)

var (
	prURL                     = flag.String("url", "", "The url of the pull request to initialize.")
	path                      = flag.String("path", "", "Path of directory under which PR will be copied")
	mode                      = flag.String("mode", "download", "Whether to operate in download or upload mode")
	provider                  = flag.String("provider", "", "The SCM provider to use. Optional")
	skipTLSVerify             = flag.Bool("insecure-skip-tls-verify", false, "Enable skipping TLS certificate verification in the git client. Defaults to false")
	disableStrictJSONComments = flag.Bool("disable-strict-json-comments", false, "Disable strict json parsing for comment files with .json extension")
)

func main() {
	flag.Parse()
	prod, _ := zap.NewProduction()
	logger := prod.Sugar()
	logger = logger.With(
		zap.String("resource_type", "pullrequest"),
		zap.String("mode", *mode))
	defer func() {
		_ = logger.Sync()
	}()
	ctx := context.Background()

	token := os.Getenv("AUTH_TOKEN")
	client, err := pullrequest.NewSCMHandler(logger, *prURL, *provider, token, *skipTLSVerify)
	if err != nil {
		logger.Fatalf("error creating GitHub client: %v", err)
	}

	switch *mode {
	case "download":
		logger.Info("RUNNING DOWNLOAD!")
		pr, err := client.Download(ctx)
		if err != nil {
			fmt.Println(err)
			logger.Fatal(err)
		}
		if err := pullrequest.ToDisk(pr, *path); err != nil {
			logger.Fatal(err)
		}

	case "upload":
		logger.Info("RUNNING UPLOAD!")
		r, err := pullrequest.FromDisk(*path, *disableStrictJSONComments)
		if err != nil {
			logger.Fatal(err)
		}
		if err := client.Upload(ctx, r); err != nil {
			logger.Fatal(err)
		}
	}
}
