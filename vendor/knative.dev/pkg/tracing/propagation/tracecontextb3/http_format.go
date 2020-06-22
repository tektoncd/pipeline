/*
Copyright 2020 The Knative Authors

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

package tracecontextb3

import (
	"go.opencensus.io/plugin/ochttp/propagation/b3"
	"go.opencensus.io/plugin/ochttp/propagation/tracecontext"
	ocpropagation "go.opencensus.io/trace/propagation"
	"knative.dev/pkg/tracing/propagation"
)

// TraceContextB3Egress is a propagation.HTTPFormat that reads both TraceContext and B3 tracing
// formats, preferring TraceContext. It always writes both formats.
var TraceContextB3Egress = &propagation.HTTPFormatSequence{
	Ingress: []ocpropagation.HTTPFormat{
		&tracecontext.HTTPFormat{},
		&b3.HTTPFormat{},
	},
	Egress: []ocpropagation.HTTPFormat{
		&tracecontext.HTTPFormat{},
		&b3.HTTPFormat{},
	},
}

// TraceContextEgress is a propagation.HTTPFormat that reads both TraceContext and B3 tracing
// formats, preferring TraceContext. It always writes TraceContext format exclusively.
var TraceContextEgress = &propagation.HTTPFormatSequence{
	Ingress: []ocpropagation.HTTPFormat{
		&tracecontext.HTTPFormat{},
		&b3.HTTPFormat{},
	},
	Egress: []ocpropagation.HTTPFormat{
		&tracecontext.HTTPFormat{},
	},
}

// B3Egress is a propagation.HTTPFormat that reads both TraceContext and B3 tracing formats,
// preferring TraceContext. It always writes B3 format exclusively.
var B3Egress = &propagation.HTTPFormatSequence{
	Ingress: []ocpropagation.HTTPFormat{
		&tracecontext.HTTPFormat{},
		&b3.HTTPFormat{},
	},
	Egress: []ocpropagation.HTTPFormat{
		&b3.HTTPFormat{},
	},
}
