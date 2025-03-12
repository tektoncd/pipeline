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
	"k8s.io/apimachinery/pkg/util/sets"
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

type (
	admissionToValue  func(*admissionv1.AdmissionRequest, *admissionv1.AdmissionResponse) string
	conversionToValue func(*apixv1.ConversionRequest, *apixv1.ConversionResponse) string
)

var (
	allAdmissionTags = map[tag.Key]admissionToValue{
		requestOperationKey: func(req *admissionv1.AdmissionRequest, _ *admissionv1.AdmissionResponse) string {
			return string(req.Operation)
		},
		kindGroupKey: func(req *admissionv1.AdmissionRequest, _ *admissionv1.AdmissionResponse) string {
			return req.Kind.Group
		},
		kindVersionKey: func(req *admissionv1.AdmissionRequest, _ *admissionv1.AdmissionResponse) string {
			return req.Kind.Version
		},
		kindKindKey: func(req *admissionv1.AdmissionRequest, _ *admissionv1.AdmissionResponse) string {
			return req.Kind.Kind
		},
		resourceGroupKey: func(req *admissionv1.AdmissionRequest, _ *admissionv1.AdmissionResponse) string {
			return req.Resource.Group
		},
		resourceVersionKey: func(req *admissionv1.AdmissionRequest, _ *admissionv1.AdmissionResponse) string {
			return req.Resource.Version
		},
		resourceResourceKey: func(req *admissionv1.AdmissionRequest, _ *admissionv1.AdmissionResponse) string {
			return req.Resource.Resource
		},
		resourceNamespaceKey: func(req *admissionv1.AdmissionRequest, _ *admissionv1.AdmissionResponse) string {
			return req.Namespace
		},
		admissionAllowedKey: func(_ *admissionv1.AdmissionRequest, resp *admissionv1.AdmissionResponse) string {
			return strconv.FormatBool(resp.Allowed)
		},
	}
	allConversionTags = map[tag.Key]conversionToValue{
		desiredAPIVersionKey: func(req *apixv1.ConversionRequest, _ *apixv1.ConversionResponse) string {
			return req.DesiredAPIVersion
		},
		resultStatusKey: func(_ *apixv1.ConversionRequest, resp *apixv1.ConversionResponse) string {
			return resp.Result.Status
		},
		resultReasonKey: func(_ *apixv1.ConversionRequest, resp *apixv1.ConversionResponse) string {
			return string(resp.Result.Reason)
		},
		resultCodeKey: func(_ *apixv1.ConversionRequest, resp *apixv1.ConversionResponse) string {
			return strconv.Itoa(int(resp.Result.Code))
		},
	}
)

// StatsReporter reports webhook metrics
type StatsReporter interface {
	ReportAdmissionRequest(request *admissionv1.AdmissionRequest, response *admissionv1.AdmissionResponse, d time.Duration) error
	ReportConversionRequest(request *apixv1.ConversionRequest, response *apixv1.ConversionResponse, d time.Duration) error
}

type statsReporterOptions struct {
	tagsToExclude sets.Set[string]
}

type StatsReporterOption func(_ *statsReporterOptions)

func WithoutTags(tags ...string) StatsReporterOption {
	return func(opts *statsReporterOptions) {
		opts.tagsToExclude.Insert(tags...)
	}
}

// reporter implements StatsReporter interface
type reporter struct {
	ctx context.Context

	admissionTags  map[tag.Key]admissionToValue
	conversionTags map[tag.Key]conversionToValue
}

// NewStatsReporter creates a reporter for webhook metrics
func NewStatsReporter(opts ...StatsReporterOption) (StatsReporter, error) {
	ctx, err := tag.New(
		context.Background(),
	)
	if err != nil {
		return nil, err
	}

	options := statsReporterOptions{
		tagsToExclude: sets.New[string](),
	}
	for _, opt := range opts {
		opt(&options)
	}

	admissionTags := make(map[tag.Key]admissionToValue)
	for key, f := range allAdmissionTags {
		if options.tagsToExclude.Has(key.Name()) {
			continue
		}
		admissionTags[key] = f
	}
	conversionTags := make(map[tag.Key]conversionToValue)
	for key, f := range allConversionTags {
		if options.tagsToExclude.Has(key.Name()) {
			continue
		}
		conversionTags[key] = f
	}

	return &reporter{
		ctx:            ctx,
		admissionTags:  admissionTags,
		conversionTags: conversionTags,
	}, nil
}

// Captures req count metric, recording the count and the duration
func (r *reporter) ReportAdmissionRequest(req *admissionv1.AdmissionRequest, resp *admissionv1.AdmissionResponse, d time.Duration) error {
	mutators := make([]tag.Mutator, 0, len(r.admissionTags))

	for key, f := range r.admissionTags {
		mutators = append(mutators, tag.Insert(key, f(req, resp)))
	}

	ctx, err := tag.New(r.ctx, mutators...)
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
	mutators := make([]tag.Mutator, 0, len(r.conversionTags))

	for key, f := range r.conversionTags {
		mutators = append(mutators, tag.Insert(key, f(req, resp)))
	}

	ctx, err := tag.New(r.ctx, mutators...)
	if err != nil {
		return err
	}

	metrics.RecordBatch(ctx, requestCountM.M(1),
		// Convert time.Duration in nanoseconds to milliseconds
		responseTimeInMsecM.M(float64(d.Milliseconds())))
	return nil
}

func RegisterMetrics(opts ...StatsReporterOption) {
	options := statsReporterOptions{
		tagsToExclude: sets.New[string](),
	}
	for _, opt := range opts {
		opt(&options)
	}

	tagKeys := []tag.Key{}
	for tag := range allAdmissionTags {
		if options.tagsToExclude.Has(tag.Name()) {
			continue
		}
		tagKeys = append(tagKeys, tag)
	}
	for tag := range allConversionTags {
		if options.tagsToExclude.Has(tag.Name()) {
			continue
		}
		tagKeys = append(tagKeys, tag)
	}

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
