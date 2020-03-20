package traceparent

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
)

const (
	// Version represents the maximum `traceparent` header version that is fully supported.
	// The library attempts optimistic forwards compatibility with higher versions.
	Version = 0
)

var (
	// ErrInvalidFormat occurs when the format is invalid, such as if there are missing characters
	// or a field contains an unexpected character set.
	ErrInvalidFormat = errors.New("tracecontext: Invalid traceparent format")
	// ErrInvalidVersion occurs when the encoded version is invalid, i.e., is 255.
	ErrInvalidVersion = errors.New("tracecontext: Invalid traceparent version")
	// ErrInvalidTraceID occurs when the encoded trace ID is invalid, i.e., all bytes are 0
	ErrInvalidTraceID = errors.New("tracecontext: Invalid traceparent trace ID")
	// ErrInvalidSpanID occurs when the encoded span ID is invalid, i.e., all bytes are 0
	ErrInvalidSpanID = errors.New("tracecontext: Invalid traceparent span ID")
)

const (
	maxVersion = 254

	numVersionBytes = 1
	numTraceIDBytes = 16
	numSpanIDBytes  = 8
	numFlagBytes    = 1
)

var (
	re = regexp.MustCompile(`^([a-f0-9]{2})-([a-f0-9]{32})-([a-f0-9]{16})-([a-f0-9]{2})(-.*)?$`)

	invalidTraceIDAllZeroes = make([]byte, numTraceIDBytes, numTraceIDBytes)
	invalidSpanIDAllZeroes  = make([]byte, numSpanIDBytes, numSpanIDBytes)
)

// Flags contain recommendations from the caller relevant to the whole trace, e.g., for sampling.
type Flags struct {
	// Recorded indicates that at least one span in the trace may have been recorded.
	// Tracing systems are advised to record all new spans in recorded traces, as incomplete traces may lead to
	// a degraded tracing experience.
	Recorded bool
}

// String encodes the Flags in an 8-bit field.
func (f Flags) String() string {
	var flags [1]byte
	if f.Recorded {
		flags[0] = 1
	}
	return fmt.Sprintf("%02x", flags)
}

// TraceParent indicates information about a span and the trace of which it is part,
// so that child spans started in the same trace may propagate necessary data and share relevant behaviour.
type TraceParent struct {
	// Version represents the version used to encode the `TraceParent`.
	// Typically, this is the minimum of this library's supported version and the version of the header from which the `TraceParent` was decoded.
	Version uint8
	// TraceID is the trace ID of the whole trace, and should be constant across all spans in a given trace.
	// A `TraceID` that contains only 0 bytes should be treated as invalid.
	TraceID [16]byte
	// SpanID is the span ID of the span from which the `TraceParent` was derived, i.e., the parent of the next span that will be started.
	// Span IDs should be unique within a given trace.
	// A `TraceID` that contains only 0 bytes should be treated as invalid.
	SpanID [8]byte
	// Flags indicate behaviour that is recommended when handling new spans.
	Flags Flags
}

// String encodes the `TraceParent` into a string formatted according to the W3C spec.
// The string may be invalid if any fields are invalid, e.g., if the `TraceID` contains only 0 bytes.
func (tp TraceParent) String() string {
	return fmt.Sprintf("%02x-%032x-%016x-%s", tp.Version, tp.TraceID, tp.SpanID, tp.Flags)
}

// Parse attempts to decode a `TraceParent` from a byte array.
// It returns an error if the byte array is incorrectly formatted or otherwise invalid.
func Parse(b []byte) (TraceParent, error) {
	return parse(b)
}

// ParseString attempts to decode a `TraceParent` from a string.
// It returns an error if the string is incorrectly formatted or otherwise invalid.
func ParseString(s string) (TraceParent, error) {
	return parse([]byte(s))
}

func parse(b []byte) (tp TraceParent, err error) {
	matches := re.FindSubmatch(b)
	if len(matches) < 6 {
		err = ErrInvalidFormat
		return
	}

	var version uint8
	if version, err = parseVersion(matches[1]); err != nil {
		return
	}
	if version == Version && len(matches[5]) > 0 {
		err = ErrInvalidFormat
		return
	}

	var traceID [16]byte
	if traceID, err = parseTraceID(matches[2]); err != nil {
		return
	}

	var spanID [8]byte
	if spanID, err = parseSpanID(matches[3]); err != nil {
		return
	}

	var flags Flags
	if flags, err = parseFlags(matches[4]); err != nil {
		return
	}

	tp.Version = Version
	tp.TraceID = traceID
	tp.SpanID = spanID
	tp.Flags = flags

	return tp, nil
}

func parseVersion(b []byte) (uint8, error) {
	version, ok := parseEncodedSegment(b, numVersionBytes)
	if !ok {
		return 0, ErrInvalidFormat
	}
	if version[0] > maxVersion {
		return 0, ErrInvalidVersion
	}
	return version[0], nil
}

func parseTraceID(b []byte) (traceID [16]byte, err error) {
	id, ok := parseEncodedSegment(b, numTraceIDBytes)
	if !ok {
		return traceID, ErrInvalidFormat
	}
	if bytes.Equal(id, invalidTraceIDAllZeroes) {
		return traceID, ErrInvalidTraceID
	}

	copy(traceID[:], id)

	return traceID, nil
}

func parseSpanID(b []byte) (spanID [8]byte, err error) {
	id, ok := parseEncodedSegment(b, numSpanIDBytes)
	if !ok {
		return spanID, ErrInvalidFormat
	}
	if bytes.Equal(id, invalidSpanIDAllZeroes) {
		return spanID, ErrInvalidSpanID
	}

	copy(spanID[:], id)

	return spanID, nil
}

func parseFlags(b []byte) (Flags, error) {
	flags, ok := parseEncodedSegment(b, numFlagBytes)
	if !ok {
		return Flags{}, ErrInvalidFormat
	}

	return Flags{
		Recorded: (flags[0] & 1) == 1,
	}, nil
}

func parseEncodedSegment(src []byte, expectedLen int) ([]byte, bool) {
	dst := make([]byte, hex.DecodedLen(len(src)))
	if n, err := hex.Decode(dst, src); n != expectedLen || err != nil {
		return dst, false
	}
	return dst, true
}
