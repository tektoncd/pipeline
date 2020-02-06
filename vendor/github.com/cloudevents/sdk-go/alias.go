package cloudevents

// Package cloudevents alias' common functions and types to improve discoverability and reduce
// the number of imports for simple HTTP clients.

import (
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/context"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
)

// Client

type ClientOption client.Option
type Client = client.Client
type ConvertFn = client.ConvertFn

// Event

type Event = cloudevents.Event
type EventResponse = cloudevents.EventResponse

// Context

type EventContext = cloudevents.EventContext
type EventContextV1 = cloudevents.EventContextV1
type EventContextV01 = cloudevents.EventContextV01
type EventContextV02 = cloudevents.EventContextV02
type EventContextV03 = cloudevents.EventContextV03

// Custom Types

type Timestamp = types.Timestamp
type URLRef = types.URLRef

// HTTP Transport

type HTTPOption http.Option
type HTTPTransport = http.Transport
type HTTPTransportContext = http.TransportContext
type HTTPTransportResponseContext = http.TransportResponseContext
type HTTPEncoding = http.Encoding

const (
	// Encoding

	ApplicationXML                  = cloudevents.ApplicationXML
	ApplicationJSON                 = cloudevents.ApplicationJSON
	ApplicationCloudEventsJSON      = cloudevents.ApplicationCloudEventsJSON
	ApplicationCloudEventsBatchJSON = cloudevents.ApplicationCloudEventsBatchJSON
	Base64                          = cloudevents.Base64

	// Event Versions

	VersionV1  = cloudevents.CloudEventsVersionV1
	VersionV01 = cloudevents.CloudEventsVersionV01
	VersionV02 = cloudevents.CloudEventsVersionV02
	VersionV03 = cloudevents.CloudEventsVersionV03

	// HTTP Transport Encodings

	HTTPBinaryV1      = http.BinaryV1
	HTTPStructuredV1  = http.StructuredV1
	HTTPBatchedV1     = http.BatchedV1
	HTTPBinaryV01     = http.BinaryV01
	HTTPStructuredV01 = http.StructuredV01
	HTTPBinaryV02     = http.BinaryV02
	HTTPStructuredV02 = http.StructuredV02
	HTTPBinaryV03     = http.BinaryV03
	HTTPStructuredV03 = http.StructuredV03
	HTTPBatchedV03    = http.BatchedV03

	// Context HTTP Transport Encodings

	Binary     = http.Binary
	Structured = http.Structured
)

var (
	// ContentType Helpers

	StringOfApplicationJSON                 = cloudevents.StringOfApplicationJSON
	StringOfApplicationXML                  = cloudevents.StringOfApplicationXML
	StringOfApplicationCloudEventsJSON      = cloudevents.StringOfApplicationCloudEventsJSON
	StringOfApplicationCloudEventsBatchJSON = cloudevents.StringOfApplicationCloudEventsBatchJSON
	StringOfBase64                          = cloudevents.StringOfBase64

	// Client Creation

	NewClient        = client.New
	NewDefaultClient = client.NewDefault

	// Client Options

	WithEventDefaulter  = client.WithEventDefaulter
	WithUUIDs           = client.WithUUIDs
	WithTimeNow         = client.WithTimeNow
	WithConverterFn     = client.WithConverterFn
	WithDataContentType = client.WithDataContentType

	// Event Creation

	NewEvent = cloudevents.New

	// Tracing

	EnableTracing = observability.EnableTracing

	// Context

	ContextWithTarget   = context.WithTarget
	TargetFromContext   = context.TargetFrom
	ContextWithEncoding = context.WithEncoding
	EncodingFromContext = context.EncodingFrom

	// Custom Types

	ParseTimestamp = types.ParseTimestamp
	ParseURLRef    = types.ParseURLRef
	ParseURIRef    = types.ParseURIRef
	ParseURI       = types.ParseURI

	// HTTP Transport

	NewHTTPTransport = http.New

	// HTTP Transport Options

	WithTarget               = http.WithTarget
	WithMethod               = http.WithMethod
	WitHHeader               = http.WithHeader
	WithShutdownTimeout      = http.WithShutdownTimeout
	WithEncoding             = http.WithEncoding
	WithContextBasedEncoding = http.WithContextBasedEncoding
	WithBinaryEncoding       = http.WithBinaryEncoding
	WithStructuredEncoding   = http.WithStructuredEncoding
	WithPort                 = http.WithPort
	WithPath                 = http.WithPath
	WithMiddleware           = http.WithMiddleware
	WithLongPollTarget       = http.WithLongPollTarget
	WithListener             = http.WithListener

	// HTTP Context

	HTTPTransportContextFrom = http.TransportContextFrom
	ContextWithHeader        = http.ContextWithHeader
)
