package zapdriver

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewProductionEncoderConfig returns an opinionated EncoderConfig for
// production environments.
func NewProductionEncoderConfig() zapcore.EncoderConfig {
	return encoderConfig
}

// NewDevelopmentEncoderConfig returns an opinionated EncoderConfig for
// development environments.
func NewDevelopmentEncoderConfig() zapcore.EncoderConfig {
	return encoderConfig
}

// NewProductionConfig is a reasonable production logging configuration.
// Logging is enabled at InfoLevel and above.
//
// It uses a JSON encoder, writes to standard error, and enables sampling.
// Stacktraces are automatically included on logs of ErrorLevel and above.
func NewProductionConfig() zap.Config {
	return zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.InfoLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "json",
		EncoderConfig:    NewProductionEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
}

// NewDevelopmentConfig is a reasonable development logging configuration.
// Logging is enabled at DebugLevel and above.
//
// It enables development mode (which makes DPanicLevel logs panic), uses a
// console encoder, writes to standard error, and disables sampling.
// Stacktraces are automatically included on logs of WarnLevel and above.
func NewDevelopmentConfig() zap.Config {
	return zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:      true,
		Encoding:         "json",
		EncoderConfig:    NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
}
