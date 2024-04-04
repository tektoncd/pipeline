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
	"fmt"
	"log"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	log.Print("Hello world received a request.")
	fmt.Fprintf(w, "Hello World! \n")
}

func main() {
	log.Print("Hello world sample started.")

	http.HandleFunc("/", handler)
	//nolint:gosec
	// #nosec G114 -- see https://github.com/securego/gosec#available-rules
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
