/*
Copyright 2017 Google Inc. All Rights Reserved.
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
	"os"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/tektoncd/pipeline/test/logs/color"
	"github.com/tektoncd/pipeline/test/logs/pipelinerun"
	trlogs "github.com/tektoncd/pipeline/test/logs/taskrun"

	"k8s.io/client-go/tools/clientcmd"
)

var (
	ns = flag.String("n", "default", "The namespace scope for this CLI request")
	pr = flag.String("pr", "", "Name of pipelinerun to tail the logs")
	tr = flag.String("tr", "", "Name of task run to tail the logs")
	f  = flag.String("f", "", "Name of the file to write logs")
)

func main() {
	flag.Parse()

	// set default output as std out
	out := os.Stdout

	if (*pr == "" && *tr == "") || (*pr != "" && *tr != "") {
		errorMsg(fmt.Sprintf("Usage: %s [-n NAMESPACE] [-pr PIPELINERUN-NAME] / [-tr TASKRUN_NAME] [-f FILE-NAME]", os.Args[0]), nil, out)
		fmt.Fprintln(out, color.Green("Note: Please provide either `-pr` or `-tr` but not both"))
		return
	}
	if *f != "" {
		logFile, err := os.OpenFile(*f, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			errorMsg("Unable to create a file", err, out)
			return
		}
		// override output with file pointer
		out = logFile
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})

	cfg, err := kubeConfig.ClientConfig()
	if err != nil {
		errorMsg("getting kube client config", err, out)
		return
	}

	ctx := context.Background()
	if *pr != "" {
		fmt.Fprintln(out, color.Green("Tailing logs for pipelinerun:")+" "+color.Green(*pr)+"...")
		if err := pipelinerun.TailLogs(ctx, cfg, out, *pr, *ns); err != nil {
			errorMsg(fmt.Sprintf("Error tailing logs for %s", *pr), err, out)
			return
		}
	}
	if *tr != "" {
		fmt.Fprintln(out, color.Green("Tailing logs for taskrun:")+" "+color.Green(*tr)+"...")
		if err := trlogs.TailLogs(ctx, cfg, *tr, *ns, out); err != nil {
			errorMsg(fmt.Sprintf("Error tailing logs for %s", *pr), err, out)
			return
		}
	}
}

func errorMsg(msg string, err error, out io.Writer) {
	var errMsg string
	if err != nil {
		errMsg = fmt.Sprintf("%s: %s", msg, err.Error())
	} else {
		errMsg = msg
	}
	fmt.Fprintln(out, color.Red(errMsg))
}
