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
	"bytes"
	"flag"
	"os"
	"os/exec"

	"github.com/golang/glog"
)

var (
	url      = flag.String("url", "", "The url of the Git repository to initialize.")
	revision = flag.String("revision", "", "The Git revision to make the repository HEAD")
)

func runOrFail(cmd string, args ...string) {
	c := exec.Command(cmd, args...)
	var output bytes.Buffer
	c.Stderr = &output
	c.Stdout = &output
	if err := c.Run(); err != nil {
		glog.Fatalf("Unexpected error running %v %v: %v\n%v", cmd, args, err, output.String())
	}
}

func main() {
	flag.Parse()

	// HACK HACK HACK
	// Git seems to ignore $HOME/.ssh and look in /root/.ssh for unknown reasons.
	// As a workaround, symlink /root/.ssh to where we expect the $HOME to land.
	// This means SSH auth only works for our built-in git support, and not
	// custom steps.
	err := os.Symlink("/builder/home/.ssh", "/root/.ssh")
	if err != nil {
		glog.Fatalf("Unexpected error creating symlink: %v", err)
	}

	runOrFail("git", "init")
	runOrFail("git", "remote", "add", "origin", *url)
	runOrFail("git", "fetch", "--depth=1", "--recurse-submodules=yes", "origin", *revision)
	runOrFail("git", "reset", "--hard", "FETCH_HEAD")

	glog.Infof("Successfully cloned %q @ %q", *url, *revision)
}
