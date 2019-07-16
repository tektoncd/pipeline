package http

// Encoding to use for HTTP transport.
type Encoding int32

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
	// Unknown is unknown.
	Unknown
)

// String pretty-prints the encoding as a string.
func (e Encoding) String() string {
	switch e {
	case Default:
		return "Default Encoding " + e.Version()

	// Binary
	case BinaryV01:
		fallthrough
	case BinaryV02:
		fallthrough
	case BinaryV03:
		return "Binary Encoding " + e.Version()

	// Structured
	case StructuredV01:
		fallthrough
	case StructuredV02:
		fallthrough
	case StructuredV03:
		return "Structured Encoding " + e.Version()

	// Batched
	case BatchedV03:
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
	case BinaryV01:
		fallthrough
	case StructuredV01:
		return "v0.1"

	// Version 0.2
	case BinaryV02:
		fallthrough
	case StructuredV02:
		return "v0.2"

	// Version 0.3
	case BinaryV03:
		fallthrough
	case StructuredV03:
		fallthrough
	case BatchedV03:
		return "v0.3"

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
		return "binary/v0.3"
	case StructuredV02:
		return "structured/v0.2"

	// Version 0.3
	case BinaryV03:
		return "binary/v0.3"
	case StructuredV03:
		return "structured/v0.3"
	case BatchedV03:
		return "batched/v0.3"

	// Unknown
	default:
		return "unknown"
	}
}
