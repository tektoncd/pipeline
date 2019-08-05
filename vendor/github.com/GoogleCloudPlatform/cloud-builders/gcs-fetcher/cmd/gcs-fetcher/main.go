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
	"time"

	"github.com/GoogleCloudPlatform/cloud-builders/gcs-fetcher/pkg/common"
	"github.com/GoogleCloudPlatform/cloud-builders/gcs-fetcher/pkg/fetcher"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

const (
	stagingFolder = ".download/"
	userAgent     = "gcs-fetcher"
)

var (
	sourceType = flag.String("type", "", "Type of source to fetch; one of Manifest, ZipArchive or TarGzArchive")
	location   = flag.String("location", "", "Location of source to fetch; in the form gs://bucket/path/to/object#generation")

	destDir     = flag.String("dest_dir", "", "The root where to write the files.")
	workerCount = flag.Int("workers", 200, "The number of files to fetch in parallel.")
	verbose     = flag.Bool("verbose", false, "If true, additional output is logged.")
	retries     = flag.Int("retries", 3, "Number of times to retry a failed GCS download.")
	backoff     = flag.Duration("backoff", 100*time.Millisecond, "Time to wait when retrying, will be doubled on each retry.")
	timeoutGCS  = flag.Bool("timeout_gcs", true, "If true, a timeout will be used to avoid GCS longtails.")
	help        = flag.Bool("help", false, "If true, prints help text and exits.")
)

func logFatalf(writer io.Writer, format string, a ...interface{}) {
	if _, err := fmt.Fprintf(writer, format+"\n", a...); err != nil {
		log.Fatalf("Failed to write log: %v", err)
	}
	os.Exit(1)
}

func main() {
	flag.Parse()

	if *help {
		fmt.Println("Fetches source files from Google Cloud Storage")
		flag.PrintDefaults()
		return
	}

	var stdout, stderr io.Writer = os.Stdout, os.Stderr
	if outputDir, ok := os.LookupEnv("BUILDER_OUTPUT"); ok {
		if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
			logFatalf(os.Stderr, "Failed to create folder %s: %v", outputDir, err)
		}
		outfile := filepath.Join(outputDir, "output")
		f, err := os.OpenFile(outfile, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			logFatalf(os.Stderr, "Cannot open output file %s: %v", outfile, err)
		}
		defer func() {
			if cerr := f.Close(); cerr != nil {
				log.Fatalf("Failed to close %q: %v", outfile, cerr)
			}
		}()
		stderr = io.MultiWriter(stderr, f)
	}

	if *location == "" || *sourceType == "" {
		logFatalf(stderr, "Must specify --location and --type")
	}

	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithUserAgent(userAgent))
	if err != nil {
		logFatalf(stderr, "Failed to create new GCS client: %v", err)
	}

	bucket, object, generation, err := common.ParseBucketObject(*location)
	if err != nil {
		logFatalf(stderr, "Failed to parse --location: %v", err)
	}

	gcs := &fetcher.Fetcher{
		GCS:         realGCS{client},
		OS:          realOS{},
		DestDir:     *destDir,
		StagingDir:  filepath.Join(*destDir, stagingFolder),
		CreatedDirs: map[string]bool{},
		Bucket:      bucket,
		Object:      object,
		Generation:  generation,
		TimeoutGCS:  *timeoutGCS,
		WorkerCount: *workerCount,
		Retries:     *retries,
		Backoff:     *backoff,
		SourceType:  *sourceType,
		Verbose:     *verbose,
		Stdout:      stdout,
		Stderr:      stderr,
	}
	if err := gcs.Fetch(ctx); err != nil {
		logFatalf(stderr, "failed to Fetch: %v", err.Error())
	}
}

// realGCS is a wrapper over the GCS client functions.
type realGCS struct {
	client *storage.Client
}

func (gp realGCS) NewReader(ctx context.Context, bucket, object string) (io.ReadCloser, error) {
	return gp.client.Bucket(bucket).Object(object).NewReader(ctx)
}

// realOS merely wraps the os package implementations.
type realOS struct{}

func (realOS) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (realOS) Chmod(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
}

func (realOS) Create(name string) (*os.File, error) {
	return os.Create(name)
}

func (realOS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (realOS) Open(name string) (*os.File, error) {
	return os.Open(name)
}

func (realOS) RemoveAll(path string) error {
	return os.RemoveAll(path)
}
