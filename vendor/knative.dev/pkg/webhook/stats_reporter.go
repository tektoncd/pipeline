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

package webhook

import (
	"context"
	"strconv"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	admissionv1 "k8s.io/api/admission/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"knative.dev/pkg/metrics"
)

const (
	requestCountName     = "request_count"
	requestLatenciesName = "request_latencies"
)

var (
	requestCountM = stats.Int64(
		requestCountName,
		"The number of requests that are routed to webhook",
		stats.UnitDimensionless)
	responseTimeInMsecM = stats.Float64(
		requestLatenciesName,
		"The response time in milliseconds",
		stats.UnitMilliseconds)

	// Create the tag keys that will be used to add tags to our measurements.
	// Tag keys must conform to the restrictions described in
	// go.opencensus.io/tag/validate.go. Currently those restrictions are:
	// - length between 1 and 255 inclusive
	// - characters are printable US-ASCII
	requestOperationKey  = tag.MustNewKey("request_operation")
	kindGroupKey         = tag.MustNewKey("kind_group")
	kindVersionKey       = tag.MustNewKey("kind_version")
	kindKindKey          = tag.MustNewKey("kind_kind")
	resourceGroupKey     = tag.MustNewKey("resource_group")
	resourceVersionKey   = tag.MustNewKey("resource_version")
	resourceResourceKey  = tag.MustNewKey("resource_resource")
	resourceNamespaceKey = tag.MustNewKey("resource_namespace")
	admissionAllowedKey  = tag.MustNewKey("admission_allowed")

	desiredAPIVersionKey = tag.MustNewKey("desired_api_version")
	resultStatusKey      = tag.MustNewKey("result_status")
	resultReasonKey      = tag.MustNewKey("result_reason")
	resultCodeKey        = tag.MustNewKey("result_code")
)

// StatsReporter reports webhook metrics
type StatsReporter interface {
	ReportAdmissionRequest(request *admissionv1.AdmissionRequest, response *admissionv1.AdmissionResponse, d time.Duration) error
	ReportConversionRequest(request *apixv1.ConversionRequest, response *apixv1.ConversionResponse, d time.Duration) error
}

// reporter implements StatsReporter interface
type reporter struct {
	ctx context.Context
}

// NewStatsReporter creates a reporter for webhook metrics
func NewStatsReporter() (StatsReporter, error) {
	ctx, err := tag.New(
		context.Background(),
	)
	if err != nil {
		return nil, err
	}

	return &reporter{ctx: ctx}, nil
}

// Captures req count metric, recording the count and the duration
func (r *reporter) ReportAdmissionRequest(req *admissionv1.AdmissionRequest, resp *admissionv1.AdmissionResponse, d time.Duration) error {
	ctx, err := tag.New(
		r.ctx,
		tag.Insert(requestOperationKey, string(req.Operation)),
		tag.Insert(kindGroupKey, req.Kind.Group),
		tag.Insert(kindVersionKey, req.Kind.Version),
		tag.Insert(kindKindKey, req.Kind.Kind),
		tag.Insert(resourceGroupKey, req.Resource.Group),
		tag.Insert(resourceVersionKey, req.Resource.Version),
		tag.Insert(resourceResourceKey, req.Resource.Resource),
		tag.Insert(resourceNamespaceKey, req.Namespace),
		tag.Insert(admissionAllowedKey, strconv.FormatBool(resp.Allowed)),
	)
	if err != nil {
		return err
	}

	metrics.RecordBatch(ctx, requestCountM.M(1),
		// Convert time.Duration in nanoseconds to milliseconds
		responseTimeInMsecM.M(float64(d.Milliseconds())))
	return nil
}

// Captures req count metric, recording the count and the duration
func (r *reporter) ReportConversionRequest(req *apixv1.ConversionRequest, resp *apixv1.ConversionResponse, d time.Duration) error {
	ctx, err := tag.New(
		r.ctx,
		tag.Insert(desiredAPIVersionKey, req.DesiredAPIVersion),
		tag.Insert(resultStatusKey, resp.Result.Status),
		tag.Insert(resultReasonKey, string(resp.Result.Reason)),
		tag.Insert(resultCodeKey, strconv.Itoa(int(resp.Result.Code))),
	)
	if err != nil {
		return err
	}

	metrics.RecordBatch(ctx, requestCountM.M(1),
		// Convert time.Duration in nanoseconds to milliseconds
		responseTimeInMsecM.M(float64(d.Milliseconds())))
	return nil
}

func RegisterMetrics() {
	tagKeys := []tag.Key{
		requestOperationKey,
		kindGroupKey,
		kindVersionKey,
		kindKindKey,
		resourceGroupKey,
		resourceVersionKey,
		resourceResourceKey,
		resourceNamespaceKey,
		admissionAllowedKey,
		desiredAPIVersionKey,
		resultStatusKey,
		resultReasonKey,
		resultCodeKey}

	if err := view.Register(
		&view.View{
			Description: requestCountM.Description(),
			Measure:     requestCountM,
			Aggregation: view.Count(),
			TagKeys:     tagKeys,
		},
		&view.View{
			Description: responseTimeInMsecM.Description(),
			Measure:     responseTimeInMsecM,
			Aggregation: view.Distribution(metrics.Buckets125(1, 100000)...), // [1 2 5 10 20 50 100 200 500 1000 2000 5000 10000 20000 50000 100000]ms
			TagKeys:     tagKeys,
		},
	); err != nil {
		panic(err)
	}
}
