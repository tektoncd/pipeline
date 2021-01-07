package zapdriver

import (
	"runtime"
	"strconv"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const sourceKey = "logging.googleapis.com/sourceLocation"

// SourceLocation adds the correct Stackdriver "SourceLocation" field.
//
// see: https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#LogEntrySourceLocation
func SourceLocation(pc uintptr, file string, line int, ok bool) zap.Field {
	return zap.Object(sourceKey, newSource(pc, file, line, ok))
}

// source is the source code location information associated with the log entry,
// if any.
type source struct {
	// Optional. Source file name. Depending on the runtime environment, this
	// might be a simple name or a fully-qualified name.
	File string `json:"file"`

	// Optional. Line within the source file. 1-based; 0 indicates no line number
	// available.
	Line string `json:"line"`

	// Optional. Human-readable name of the function or method being invoked, with
	// optional context such as the class or package name. This information may be
	// used in contexts such as the logs viewer, where a file and line number are
	// less meaningful.
	//
	// The format should be dir/package.func.
	Function string `json:"function"`
}

// MarshalLogObject implements zapcore.ObjectMarshaller interface.
func (source source) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("file", source.File)
	enc.AddString("line", source.Line)
	enc.AddString("function", source.Function)

	return nil
}

func newSource(pc uintptr, file string, line int, ok bool) *source {
	if !ok {
		return nil
	}

	var function string
	if fn := runtime.FuncForPC(pc); fn != nil {
		function = fn.Name()
	}

	source := &source{
		File:     file,
		Line:     strconv.Itoa(line),
		Function: function,
	}

	return source
}
