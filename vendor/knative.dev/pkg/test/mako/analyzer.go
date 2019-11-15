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

package mako

import (
	"github.com/golang/protobuf/proto"
	tpb "github.com/google/mako/clients/proto/analyzers/threshold_analyzer_go_proto"
	mpb "github.com/google/mako/spec/proto/mako_go_proto"
)

// NewCrossRunConfig returns a config that can be used in ThresholdAnalyzer.
// By using it, the Analyzer will only fail if there are xx continuous runs that cross the threshold.
func NewCrossRunConfig(runCount int32, tags ...string) *tpb.CrossRunConfig {
	return &tpb.CrossRunConfig{
		RunInfoQueryList: []*mpb.RunInfoQuery{{
			Limit: proto.Int32(runCount),
			Tags:  tags,
		}},
		MinRunCount: proto.Int32(runCount),
	}
}
