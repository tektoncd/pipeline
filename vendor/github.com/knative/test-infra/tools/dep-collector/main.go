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

package main

import (
	"flag"
	"log"
	"os"

	"github.com/google/licenseclassifier"
)

var WorkingDir, _ = os.Getwd()

var (
	csv   = flag.Bool("csv", false, "Whether to print in CSV format (with slow classification).")
	check = flag.Bool("check", false, "Whether to just check license files for forbidden licenses.")
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatalf("Expected a list of import paths, got: %v", flag.Args())
	}

	// Perform a simple DFS to collect the binaries' transitive dependencies.
	transitiveImports, err := CollectTransitiveImports(flag.Args())
	if err != nil {
		log.Fatalf("Error collecting transitive dependencies: %v", err)
	}

	// Gather all of the license data from the imports.
	collection, err := CollectLicenses(transitiveImports)
	if err != nil {
		log.Fatalf("Error identifying licenses for transitive dependencies: %v", err)
	}

	if *check {
		classifier, err := licenseclassifier.NewWithForbiddenLicenses(MatchThreshold)
		if err != nil {
			log.Fatalf("Error creating license classifier: %v", err)
		}
		if err := collection.Check(classifier); err != nil {
			log.Fatalf("Error checking license collection: %v", err)
		}
		log.Printf("No errors found.")
		return
	}

	if *csv {
		classifier, err := licenseclassifier.New(MatchThreshold)
		if err != nil {
			log.Fatalf("Error creating license classifier: %v", err)
		}
		output, err := collection.CSV(classifier)
		if err != nil {
			log.Fatalf("Error generating CSV: %v", err)
		}
		os.Stdout.Write([]byte(output))
	} else {
		entries, err := collection.Entries()
		if err != nil {
			log.Fatalf("Error generating entries: %v", err)
		}
		os.Stdout.Write([]byte(entries))
	}
}
