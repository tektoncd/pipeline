/*
Copyright 2018 Google, Inc. All rights reserved.

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
	"io"
	"log"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"github.com/GoogleCloudPlatform/cloud-builders/gcs-fetcher/pkg/common"
	"github.com/GoogleCloudPlatform/cloud-builders/gcs-fetcher/pkg/uploader"
)

const userAgent = "gcs-uploader"

var (
	dir         = flag.String("dir", ".", "Directory of files to upload")
	location    = flag.String("location", "", "Location of manifest file to upload; in the form gs://bucket/path/to/object")
	workerCount = flag.Int("workers", 200, "The number of files to upload in parallel.")
	help        = flag.Bool("help", false, "If true, prints help text and exits.")
)

func main() {
	flag.Parse()

	if *help {
		fmt.Println("Incrementally uploads source files to Google Cloud Storage")
		flag.PrintDefaults()
		return
	}

	if *location == "" {
		log.Fatalln("Must specify --location")
	}
	bucket, object, generation, err := common.ParseBucketObject(*location)
	if err != nil {
		log.Fatalf("parsing location from %q: %v", *location, err)
	}
	if generation != 0 {
		log.Fatalln("cannot specify manifest file generation")
	}

	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithUserAgent(userAgent))
	if err != nil {
		log.Fatalf("Failed to create new GCS client: %v", err)
	}

	u := uploader.New(ctx, realGCS{client}, realOS{}, bucket, object, *workerCount)

	filepath.Walk(*dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		u.Do(ctx, path, info)
		return nil
	})

	if err := u.Done(ctx); err != nil {
		log.Fatalf("Failed to upload: %v", err)
	}
}

// realGCS is a wrapper over the GCS client functions.
type realGCS struct {
	client *storage.Client
}

func (gp realGCS) NewWriter(ctx context.Context, bucket, object string) io.WriteCloser {
	return gp.client.Bucket(bucket).Object(object).
		If(storage.Conditions{DoesNotExist: true}). // Skip upload if already exists.
		NewWriter(ctx)
}

// realOS merely wraps the os package implementations.
type realOS struct{}

func (realOS) EvalSymlinks(path string) (string, error) { return filepath.EvalSymlinks(path) }
func (realOS) Stat(path string) (os.FileInfo, error)    { return os.Stat(path) }
