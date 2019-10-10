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

package webhook

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"

	"go.uber.org/zap"
	"k8s.io/api/admission/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"knative.dev/pkg/test/webhook-apicoverage/coveragecalculator"
	"knative.dev/pkg/test/webhook-apicoverage/resourcetree"
	"knative.dev/pkg/test/webhook-apicoverage/view"
	"knative.dev/pkg/webhook"
)

var (
	decoder = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
)

const (
	// ResourceQueryParam query param name to provide the resource.
	ResourceQueryParam = "resource"

	// ResourceCoverageEndPoint is the endpoint for Resource Coverage API
	ResourceCoverageEndPoint = "/resourcecoverage"

	// TotalCoverageEndPoint is the endpoint for Total Coverage API
	TotalCoverageEndPoint = "/totalcoverage"

	// ResourcePercentageCoverageEndPoint is the end point for Resource Percentage
	// coverages API
	ResourcePercentageCoverageEndPoint = "/resourcepercentagecoverage"

	// resourceChannelQueueSize size of the queue maintained for resource channel.
	resourceChannelQueueSize = 10
)

type resourceChannelMsg struct {
	resourceType     schema.GroupVersionKind
	rawResourceValue []byte
}

// APICoverageRecorder type contains resource tree to record API coverage for resources.
type APICoverageRecorder struct {
	Logger          *zap.SugaredLogger
	ResourceForest  resourcetree.ResourceForest
	ResourceMap     map[schema.GroupVersionKind]webhook.GenericCRD
	NodeRules       resourcetree.NodeRules
	FieldRules      resourcetree.FieldRules
	DisplayRules    view.DisplayRules
	resourceChannel chan resourceChannelMsg
}

// Init initializes the resources trees for set resources.
func (a *APICoverageRecorder) Init() {
	for resourceKind, resourceObj := range a.ResourceMap {
		a.ResourceForest.AddResourceTree(resourceKind.Kind, reflect.ValueOf(resourceObj).Elem().Type())
	}
	a.resourceChannel = make(chan resourceChannelMsg, resourceChannelQueueSize)
	go a.updateResourceCoverageTree()
}

// updateResourceCoverageTree updates the resource coverage tree.
func (a *APICoverageRecorder) updateResourceCoverageTree() {
	for {
		channelMsg := <-a.resourceChannel
		if err := json.Unmarshal(channelMsg.rawResourceValue, a.ResourceMap[channelMsg.resourceType]); err != nil {
			a.Logger.Errorf("Failed unmarshalling review.Request.Object.Raw for type: %s Error: %v", channelMsg.resourceType.Kind, err)
			continue
		}
		resourceTree := a.ResourceForest.TopLevelTrees[channelMsg.resourceType.Kind]
		resourceTree.UpdateCoverage(reflect.ValueOf(a.ResourceMap[channelMsg.resourceType]).Elem())
		a.Logger.Info("Successfully recorded coverage for resource ", channelMsg.resourceType.Kind)
	}
}

// RecordResourceCoverage updates the resource tree with the request.
func (a *APICoverageRecorder) RecordResourceCoverage(w http.ResponseWriter, r *http.Request) {
	var (
		body []byte
		err  error
	)

	review := &v1beta1.AdmissionReview{}
	if body, err = ioutil.ReadAll(r.Body); err != nil {
		a.Logger.Errorf("Failed reading request body: %v", err)
		a.appendAndWriteAdmissionResponse(review, false, "Admission Denied", w)
		return
	}

	if _, _, err := decoder.Decode(body, nil, review); err != nil {
		a.Logger.Errorf("Unable to decode request: %v", err)
		a.appendAndWriteAdmissionResponse(review, false, "Admission Denied", w)
		return
	}

	gvk := schema.GroupVersionKind{
		Group:   review.Request.Kind.Group,
		Version: review.Request.Kind.Version,
		Kind:    review.Request.Kind.Kind,
	}
	// We only care about resources the repo has setup.
	if _, ok := a.ResourceMap[gvk]; !ok {
		a.Logger.Info("By-passing resource coverage update for resource : %s", gvk.Kind)
		a.appendAndWriteAdmissionResponse(review, true, "Welcome Aboard", w)
		return
	}

	a.resourceChannel <- resourceChannelMsg{
		resourceType:     gvk,
		rawResourceValue: review.Request.Object.Raw,
	}
	a.appendAndWriteAdmissionResponse(review, true, "Welcome Aboard", w)
}

func (a *APICoverageRecorder) appendAndWriteAdmissionResponse(review *v1beta1.AdmissionReview, allowed bool, message string, w http.ResponseWriter) {
	review.Response = &v1beta1.AdmissionResponse{
		Allowed: allowed,
		Result: &v1.Status{
			Message: message,
		},
	}

	responseInBytes, err := json.Marshal(review)
	if err != nil {
		a.Logger.Errorf("Failing mashalling review response: %v", err)
	}

	if _, err := w.Write(responseInBytes); err != nil {
		a.Logger.Errorf("%v", err)
	}
}

// GetResourceCoverage retrieves resource coverage data for the passed in resource via query param.
func (a *APICoverageRecorder) GetResourceCoverage(w http.ResponseWriter, r *http.Request) {
	resource := r.URL.Query().Get(ResourceQueryParam)
	if _, ok := a.ResourceForest.TopLevelTrees[resource]; !ok {
		fmt.Fprintf(w, "Resource information not found for resource: %s", resource)
		return
	}

	var ignoredFields coveragecalculator.IgnoredFields
	ignoredFieldsFilePath := os.Getenv("KO_DATA_PATH") + "/ignoredfields.yaml"
	if err := ignoredFields.ReadFromFile(ignoredFieldsFilePath); err != nil {
		a.Logger.Errorf("Error reading file %s: %v", ignoredFieldsFilePath, err)
	}

	tree := a.ResourceForest.TopLevelTrees[resource]
	typeCoverage := tree.BuildCoverageData(a.NodeRules, a.FieldRules, ignoredFields)
	coverageValues := coveragecalculator.CalculateTypeCoverage(typeCoverage)
	coverageValues.CalculatePercentageValue()

	if htmlData, err := view.GetHTMLDisplay(typeCoverage, coverageValues); err != nil {
		fmt.Fprintf(w, "Error generating html file %v", err)
	} else {
		fmt.Fprint(w, htmlData)
	}
}

// GetTotalCoverage goes over all the resources setup for the apicoverage tool and returns total coverage values.
func (a *APICoverageRecorder) GetTotalCoverage(w http.ResponseWriter, r *http.Request) {
	var (
		ignoredFields coveragecalculator.IgnoredFields
		err           error
	)

	ignoredFieldsFilePath := os.Getenv("KO_DATA_PATH") + "/ignoredfields.yaml"
	if err = ignoredFields.ReadFromFile(ignoredFieldsFilePath); err != nil {
		a.Logger.Errorf("Error reading file %s: %v", ignoredFieldsFilePath, err)
	}

	totalCoverage := coveragecalculator.CoverageValues{}
	for resource := range a.ResourceMap {
		tree := a.ResourceForest.TopLevelTrees[resource.Kind]
		typeCoverage := tree.BuildCoverageData(a.NodeRules, a.FieldRules, ignoredFields)
		coverageValues := coveragecalculator.CalculateTypeCoverage(typeCoverage)
		totalCoverage.TotalFields += coverageValues.TotalFields
		totalCoverage.CoveredFields += coverageValues.CoveredFields
		totalCoverage.IgnoredFields += coverageValues.IgnoredFields
	}

	totalCoverage.CalculatePercentageValue()
	var body []byte
	if body, err = json.Marshal(totalCoverage); err != nil {
		fmt.Fprintf(w, "error marshalling total coverage response: %v", err)
		return
	}

	if _, err = w.Write(body); err != nil {
		fmt.Fprintf(w, "error writing total coverage response: %v", err)
	}
}

// GetResourceCoveragePercentags goes over all the resources setup for the
// apicoverage tool and returns percentage coverage for each resource.
func (a *APICoverageRecorder) GetResourceCoveragePercentages(
	w http.ResponseWriter, r *http.Request) {
	var (
		ignoredFields coveragecalculator.IgnoredFields
		err           error
	)

	ignoredFieldsFilePath :=
		os.Getenv("KO_DATA_PATH") + "/ignoredfields.yaml"
	if err = ignoredFields.ReadFromFile(ignoredFieldsFilePath); err != nil {
		a.Logger.Errorf("Error reading file %s: %v",
			ignoredFieldsFilePath, err)
	}

	totalCoverage := coveragecalculator.CoverageValues{}
	percentCoverages := make(map[string]float64)
	for resource := range a.ResourceMap {
		tree := a.ResourceForest.TopLevelTrees[resource.Kind]
		typeCoverage := tree.BuildCoverageData(a.NodeRules, a.FieldRules,
			ignoredFields)
		coverageValues := coveragecalculator.CalculateTypeCoverage(typeCoverage)
		coverageValues.CalculatePercentageValue()
		percentCoverages[resource.Kind] = coverageValues.PercentCoverage
		totalCoverage.TotalFields += coverageValues.TotalFields
		totalCoverage.CoveredFields += coverageValues.CoveredFields
		totalCoverage.IgnoredFields += coverageValues.IgnoredFields
	}
	totalCoverage.CalculatePercentageValue()
	percentCoverages["Overall"] = totalCoverage.PercentCoverage

	var body []byte
	if body, err = json.Marshal(
		coveragecalculator.CoveragePercentages{
			ResourceCoverages: percentCoverages,
		}); err != nil {
		fmt.Fprintf(w, "error marshalling percentage coverage response: %v",
			err)
		return
	}

	if _, err = w.Write(body); err != nil {
		fmt.Fprintf(w, "error writing percentage coverage response: %v",
			err)
	}
}
