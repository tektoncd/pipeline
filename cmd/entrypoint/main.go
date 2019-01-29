/*
Copyright 2019 The Knative Authors.

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

	"github.com/knative/build/pkg/entrypoint"
)

var (
	ep       = flag.String("entrypoint", "", "Original specified entrypoint to execute")
	waitFile = flag.String("wait_file", "", "If specified, file to wait for")
	postFile = flag.String("post_file", "", "If specified, file to write upon completion")
)

func main() {
	flag.Parse()

	entrypoint.Entrypointer{
		Entrypoint: *ep,
		WaitFile:   *waitFile,
		PostFile:   *postFile,
		Args:       flag.Args(),
		Waiter:     &entrypoint.RealWaiter{},
		Runner:     &entrypoint.RealRunner{},
		PostWriter: &entrypoint.RealPostWriter{},
	}.Go()
}

// TODO(jasonhall): Test that original exit code is propagated and that stdout/stderr are collected -- needs e2e tests.
