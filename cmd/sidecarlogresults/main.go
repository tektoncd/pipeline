package main

import (
	"encoding/json"
	"flag"
	"github.com/tektoncd/pipeline/internal/sidecarlogresults"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/pod"
	"log"
	"os"
	"strings"
)

func main() {
	var resultsDir string
	var resultNames string
	var stepResultsStr string
	flag.StringVar(&resultsDir, "results-dir", pipeline.DefaultResultPath, "Path to the results directory. Default is /tekton/results")
	flag.StringVar(&resultNames, "result-names", "", "comma separated result names to expect from the steps running in the pod. eg. foo,bar,baz")
	flag.StringVar(&stepResultsStr, "step-results", "", "json containing a map of step Name as key and list of result Names. eg. {\"stepName\":[\"foo\",\"bar\",\"baz\"]}")
	flag.Parse()
	if resultNames == "" {
		log.Fatal("result-names were not provided")
	}
	expectedResults := strings.Split(resultNames, ",")
	expectedStepResults := map[string][]string{}
	if err := json.Unmarshal([]byte(stepResultsStr), &expectedStepResults); err != nil {
		log.Fatal(err)
	}
	err := sidecarlogresults.LookForResults(os.Stdout, pod.RunDir, resultsDir, expectedResults, pipeline.StepsDir, expectedStepResults)
	if err != nil {
		log.Fatal(err)
	}

}
