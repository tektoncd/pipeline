/*
Copyright 2019 The Knative Authors

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

package config

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/proto"
	mpb "github.com/google/mako/spec/proto/mako_go_proto"
)

const koDataPathEnvName = "KO_DATA_PATH"

// MustGetBenchmark wraps getBenchmark in log.Fatalf
func MustGetBenchmark() (*string, *string) {
	benchmarkKey, benchmarkName, err := getBenchmark()
	if err != nil {
		log.Fatalf("unable to determine benchmark_key: %v", err)
	}
	return benchmarkKey, benchmarkName
}

// getBenchmark fetches the appropriate benchmark_key for this configured environment.
func getBenchmark() (*string, *string, error) {
	// Figure out what environment we're running in from the Mako configmap.
	env, err := getEnvironment()
	if err != nil {
		return nil, nil, err
	}
	// Read the Mako config file for this environment.
	data, err := readFileFromKoData(env + ".config")
	if err != nil {
		return nil, nil, err
	}
	// Parse the Mako config file.
	bi := &mpb.BenchmarkInfo{}
	if err := proto.UnmarshalText(string(data), bi); err != nil {
		return nil, nil, err
	}

	// Return the benchmark_key from this environment's config file.
	return bi.BenchmarkKey, bi.BenchmarkName, nil
}

// readFileFromKoData reads the named file from kodata.
func readFileFromKoData(name string) ([]byte, error) {
	koDataPath := os.Getenv(koDataPathEnvName)
	if koDataPath == "" {
		return nil, fmt.Errorf("%q does not exist or is empty", koDataPathEnvName)
	}
	fullFilename := filepath.Join(koDataPath, name)
	return ioutil.ReadFile(fullFilename)
}
