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

// gcs.go defines functions on gcs

package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

const (
	logDir     = "logs/"
	sourceDir  = "ci-knative-serving-continuous"
	bucketName = "knative-prow"
)

var client *storage.Client

func createStorageClient(ctx context.Context, sa string) error {
	var err error
	client, err = storage.NewClient(ctx, option.WithCredentialsFile(sa))
	return err
}

func createStorageObject(filename string) *storage.ObjectHandle {
	return client.Bucket(bucketName).Object(filename)
}

func readGcsFile(ctx context.Context, filename string, sa string) ([]byte, error) {
	// Create a new GCS client
	if err := createStorageClient(ctx, sa); err != nil {
		log.Fatalf("Failed to create GCS client: %v", err)
	}
	o := createStorageObject(filename)
	if _, err := o.Attrs(ctx); err != nil {
		return []byte(fmt.Sprintf("Cannot get attributes of '%s'", filename)), err
	}
	f, err := o.NewReader(ctx)
	if err != nil {
		return []byte(fmt.Sprintf("Cannot open '%s'", filename)), err
	}
	defer f.Close()
	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return []byte(fmt.Sprintf("Cannot read '%s'", filename)), err
	}
	return contents, nil
}

func parseLog(ctx context.Context, dir string, isLocal bool, coverage *OverallAPICoverage) []string {
	var covLogs []string

	buildDir := logDir + dir
	log.Printf("Parsing '%s'", buildDir)
	logFile := buildDir + "/build-log.txt"
	log.Printf("Parsing '%s'", logFile)
	o := createStorageObject(logFile)
	if _, err := o.Attrs(ctx); err != nil {
		log.Printf("Cannot get attributes of '%s', assuming not ready yet: %v", logFile, err)
		return nil
	}
	f, err := o.NewReader(ctx)
	if err != nil {
		log.Fatalf("Error opening '%s': %v", logFile, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		// I0727 16:23:30.055] info	TestRouteCreation	test/configuration.go:34	resource {<resource_name>: <val>}"}
		if len(fields) == 7 && fields[2] == "info" && fields[5] == "resource" {
			covLogs = append(covLogs, strings.Join(fields[6:], " "))
		}
	}
	return covLogs
}
