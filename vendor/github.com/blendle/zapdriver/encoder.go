package zapdriver

import (
	"time"

	"go.uber.org/zap/zapcore"
)

// logLevelSeverity maps the Zap log levels to the correct level names as
// defined by Stackdriver.
//
// DEFAULT     (0) The log entry has no assigned severity level.
// DEBUG     (100) Debug or trace information.
// INFO      (200) Routine information, such as ongoing status or performance.
// NOTICE    (300) Normal but significant events, such as start up, shut down, or a configuration change.
// WARNING   (400) Warning events might cause problems.
// ERROR     (500) Error events are likely to cause problems.
// CRITICAL  (600) Critical events cause more severe problems or outages.
// ALERT     (700) A person must take an action immediately.
// EMERGENCY (800) One or more systems are unusable.
//
// See: https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#LogSeverity
var logLevelSeverity = map[zapcore.Level]string{
	zapcore.DebugLevel:  "DEBUG",
	zapcore.InfoLevel:   "INFO",
	zapcore.WarnLevel:   "WARNING",
	zapcore.ErrorLevel:  "ERROR",
	zapcore.DPanicLevel: "CRITICAL",
	zapcore.PanicLevel:  "ALERT",
	zapcore.FatalLevel:  "EMERGENCY",
}

// encoderConfig is the default encoder configuration, slightly tweaked to use
// the correct fields for Stackdriver to parse them.
var encoderConfig = zapcore.EncoderConfig{
	TimeKey:        "timestamp",
	LevelKey:       "severity",
	NameKey:        "logger",
	CallerKey:      "caller",
	MessageKey:     "message",
	StacktraceKey:  "stacktrace",
	LineEnding:     zapcore.DefaultLineEnding,
	EncodeLevel:    EncodeLevel,
	EncodeTime:     RFC3339NanoTimeEncoder,
	EncodeDuration: zapcore.SecondsDurationEncoder,
	EncodeCaller:   zapcore.ShortCallerEncoder,
}

// EncodeLevel maps the internal Zap log level to the appropriate Stackdriver
// level.
func EncodeLevel(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(logLevelSeverity[l])
}

// RFC3339NanoTimeEncoder serializes a time.Time to an RFC3339Nano-formatted
// string with nanoseconds precision.
func RFC3339NanoTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format(time.RFC3339Nano))
}
