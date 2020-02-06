package http

import (
	"context"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	cecontext "github.com/cloudevents/sdk-go/pkg/cloudevents/context"
)

// Encoding to use for HTTP transport.
type Encoding int32

type EncodingSelector func(context.Context, cloudevents.Event) Encoding

const (
	// Default
	Default Encoding = iota
	// BinaryV01 is Binary CloudEvents spec v0.1.
	BinaryV01
	// StructuredV01 is Structured CloudEvents spec v0.1.
	StructuredV01
	// BinaryV02 is Binary CloudEvents spec v0.2.
	BinaryV02
	// StructuredV02 is Structured CloudEvents spec v0.2.
	StructuredV02
	// BinaryV03 is Binary CloudEvents spec v0.3.
	BinaryV03
	// StructuredV03 is Structured CloudEvents spec v0.3.
	StructuredV03
	// BatchedV03 is Batched CloudEvents spec v0.3.
	BatchedV03
	// BinaryV1 is Binary CloudEvents spec v1.0.
	BinaryV1
	// StructuredV03 is Structured CloudEvents spec v1.0.
	StructuredV1
	// BatchedV1 is Batched CloudEvents spec v1.0.
	BatchedV1

	// Unknown is unknown.
	Unknown

	// Binary is used for Context Based Encoding Selections to use the
	// DefaultBinaryEncodingSelectionStrategy
	Binary = "binary"

	// Structured is used for Context Based Encoding Selections to use the
	// DefaultStructuredEncodingSelectionStrategy
	Structured = "structured"

	// Batched is used for Context Based Encoding Selections to use the
	// DefaultStructuredEncodingSelectionStrategy
	Batched = "batched"
)

func ContextBasedEncodingSelectionStrategy(ctx context.Context, e cloudevents.Event) Encoding {
	encoding := cecontext.EncodingFrom(ctx)
	switch encoding {
	case "", Binary:
		return DefaultBinaryEncodingSelectionStrategy(ctx, e)
	case Structured:
		return DefaultStructuredEncodingSelectionStrategy(ctx, e)
	}
	return Default
}

// DefaultBinaryEncodingSelectionStrategy implements a selection process for
// which binary encoding to use based on spec version of the event.
func DefaultBinaryEncodingSelectionStrategy(ctx context.Context, e cloudevents.Event) Encoding {
	switch e.SpecVersion() {
	case cloudevents.CloudEventsVersionV01:
		return BinaryV01
	case cloudevents.CloudEventsVersionV02:
		return BinaryV02
	case cloudevents.CloudEventsVersionV03:
		return BinaryV03
	case cloudevents.CloudEventsVersionV1:
		return BinaryV1
	}
	// Unknown version, return Default.
	return Default
}

// DefaultStructuredEncodingSelectionStrategy implements a selection process
// for which structured encoding to use based on spec version of the event.
func DefaultStructuredEncodingSelectionStrategy(ctx context.Context, e cloudevents.Event) Encoding {
	switch e.SpecVersion() {
	case cloudevents.CloudEventsVersionV01:
		return StructuredV01
	case cloudevents.CloudEventsVersionV02:
		return StructuredV02
	case cloudevents.CloudEventsVersionV03:
		return StructuredV03
	case cloudevents.CloudEventsVersionV1:
		return StructuredV1
	}
	// Unknown version, return Default.
	return Default
}

// String pretty-prints the encoding as a string.
func (e Encoding) String() string {
	switch e {
	case Default:
		return "Default Encoding " + e.Version()

	// Binary
	case BinaryV01, BinaryV02, BinaryV03, BinaryV1:
		return "Binary Encoding " + e.Version()

	// Structured
	case StructuredV01, StructuredV02, StructuredV03, StructuredV1:
		return "Structured Encoding " + e.Version()

	// Batched
	case BatchedV03, BatchedV1:
		return "Batched Encoding " + e.Version()

	default:
		return "Unknown Encoding"
	}
}

// Version pretty-prints the encoding version as a string.
func (e Encoding) Version() string {
	switch e {
	case Default:
		return "Default"

	// Version 0.1
	case BinaryV01, StructuredV01:
		return "v0.1"

	// Version 0.2
	case BinaryV02, StructuredV02:
		return "v0.2"

	// Version 0.3
	case BinaryV03, StructuredV03, BatchedV03:
		return "v0.3"

	// Version 1.0
	case BinaryV1, StructuredV1, BatchedV1:
		return "v1.0"

	// Unknown
	default:
		return "Unknown"
	}
}

// Codec creates a structured string to represent the the codec version.
func (e Encoding) Codec() string {
	switch e {
	case Default:
		return "default"

	// Version 0.1
	case BinaryV01:
		return "binary/v0.1"
	case StructuredV01:
		return "structured/v0.1"

	// Version 0.2
	case BinaryV02:
		return "binary/v0.2"
	case StructuredV02:
		return "structured/v0.2"

	// Version 0.3
	case BinaryV03:
		return "binary/v0.3"
	case StructuredV03:
		return "structured/v0.3"
	case BatchedV03:
		return "batched/v0.3"

	// Version 1.0
	case BinaryV1:
		return "binary/v1.0"
	case StructuredV1:
		return "structured/v1.0"
	case BatchedV1:
		return "batched/v1.0"

	// Unknown
	default:
		return "unknown"
	}
}

// Name creates a string to represent the the codec name.
func (e Encoding) Name() string {
	switch e {
	case Default:
		return Binary
	case BinaryV01, BinaryV02, BinaryV03, BinaryV1:
		return Binary
	case StructuredV01, StructuredV02, StructuredV03, StructuredV1:
		return Structured
	case BatchedV03, BatchedV1:
		return Batched
	default:
		return Binary
	}
}
