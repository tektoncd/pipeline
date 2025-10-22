/*
Copyright 2018 The Knative Authors

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

// logging.go contains the logic to configure and interact with the
// logging and metrics libraries.

package logging

import (
	"context"
	"flag"
	"os"
	"strconv"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/klog/v2"
)

var tracer = otel.GetTracerProvider().Tracer("knative.dev/pkg/test/logging")

// FormatLogger is a printf style function for logging in tests.
type FormatLogger func(template string, args ...interface{})

// GetEmitableSpan starts and returns a trace.Span with a name that
// is used by the ExportSpan method to emit the span.
//
//nolint:spancheck
func GetEmitableSpan(ctx context.Context, metricName string) trace.Span {
	_, span := tracer.Start(ctx, metricName)
	return span
}

const (
	logrZapDebugLevel = 3
)

func zapLevelFromLogrLevel(logrLevel int) zapcore.Level {
	// Zap levels are -1, 0, 1, 2,... corresponding to DebugLevel, InfoLevel, WarnLevel, ErrorLevel,...
	// zapr library just does zapLevel := -1*logrLevel; which means:
	//  1. Info level is only active at 0 (versus 2 in klog being generally equivalent to Info)
	//  2. Only verbosity of 0 and 1 map to valid Zap levels
	// According to https://github.com/uber-go/zap/issues/713 custom levels (i.e. < -1) aren't guaranteed to work, so not using them (for now).

	l := zap.InfoLevel
	if logrLevel >= logrZapDebugLevel {
		l = zap.DebugLevel
	}

	return l
}

func printFlags() {
	var flagList []interface{}
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		flagList = append(flagList, f.Name, f.Value.String())
	})
	logger.Sugar().Debugw("Test Flags", flagList...)
}

var (
	zapCore              zapcore.Core
	logger               *zap.Logger
	verbosity            int // Amount of log verbosity
	loggerInitializeOnce = &sync.Once{}
)

// InitializeLogger initializes logging for Knative tests.
// It should be called prior to executing tests but after command-line flags have been processed.
// It is recommended doing it in the TestMain() function.
func InitializeLogger() {
	loggerInitializeOnce.Do(func() {
		humanEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())

		// Output streams
		// TODO(coryrc): also open a log file if in Prow?
		stdOut := zapcore.Lock(os.Stdout)

		// Level function helper
		zapLevel := zapLevelFromLogrLevel(verbosity)
		isPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= zapLevel
		})

		// Assemble the output streams
		zapCore = zapcore.NewTee(
			// TODO(coryrc): log JSON output somewhere?
			zapcore.NewCore(humanEncoder, stdOut, isPriority),
		)

		logger = zap.New(zapCore)
		zap.ReplaceGlobals(logger) // Gets used by klog/glog proxy libraries

		// Set klog/glog verbosities (works with and without proxy libraries)
		klogLevel := klog.Level(0)
		klogLevel.Set(strconv.Itoa(verbosity))

		if verbosity > 2 {
			printFlags()
		}
	})
}

func init() {
	flag.IntVar(&verbosity, "verbosity", 2,
		"Amount of verbosity, 0-10. See https://github.com/go-logr/logr#how-do-i-choose-my-v-levels and https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md")
}
