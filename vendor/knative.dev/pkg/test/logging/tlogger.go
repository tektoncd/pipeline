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

package logging

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TLogger is TLogger
type TLogger struct {
	l        *zap.Logger
	level    int
	t        *testing.T
	errs     map[string][]interface{} // For Collect()
	dontFail bool
}

// V() returns an InfoLogger from go-logr.
//
// This should be the main way your tests log.
// Most frequent usage is used directly:
//   t.V(2).Info("Something at regular level")
// But if something computationally difficult is to be done, can do:
//   if l := t.V(8); l.Enabled() {
//     x := somethingExpensive()
//     l.Info("logging it", "expensiveThing", x)
//   }
//
// Elsewhere in this documentation refers to a hypothetical .V(errorLevel) to simplify explanations.
// The V() function cannot write to the error level; the Error, ErrorIfErr, Fatal, and
// FatalIfErr methods are the only way to write to the error level.
func (o *TLogger) V(level int) logr.InfoLogger {
	// Consider adding || (level <= logrZapDebugLevel && o.l.Core().Enabled(zapLevelFromLogrLevel(level)))
	// Reason to add it is even if you ask for verbosity=1, in case of error you'll get up to verbosity=3 in the debug output
	// but since zapTest uses Debug, you always get V(<=3) even when verbosity < 3
	// Probable solution is to write to t.Log at Info level?
	if level <= o.level {
		return &infoLogger{
			logrLevel: o.level,
			t:         o,
		}
	}
	return disabledInfoLogger
}

// WithValues() acts like Zap's With() method.
// Consistent with logr.Logger.WithValues()
// Whenever anything is logged with the returned TLogger,
// it will act as if these keys and values were passed into every logging call.
func (o *TLogger) WithValues(keysAndValues ...interface{}) *TLogger {
	return o.cloneWithNewLogger(o.l.With(o.handleFields(keysAndValues)...))
}

// WithName() acts like Zap's Named() method.
// Consistent with logr.Logger.WithName()
// Appends the name onto the current logger
func (o *TLogger) WithName(name string) *TLogger {
	return o.cloneWithNewLogger(o.l.Named(name))
}

// Custom additions:

// ErrorIfErr fails the current test if the err != nil.
// Remaining arguments function as if passed to .V(errorLevel).Info() (were that a thing)
// Same signature as logr.Logger.Error() method, but as this is a test, it functions slightly differently.
func (o *TLogger) ErrorIfErr(err error, msg string, keysAndValues ...interface{}) {
	if err != nil {
		o.error(err, msg, keysAndValues)
		o.fail()
	}
}

// FatalIfErr is just like ErrorIfErr() but test execution stops immediately
func (o *TLogger) FatalIfErr(err error, msg string, keysAndValues ...interface{}) {
	if err != nil {
		o.error(err, msg, keysAndValues)
		o.failNow()
	}
}

// Error is essentially a .V(errorLevel).Info() followed by failing the test.
// Intended usage is Error(msg string, key-value alternating arguments)
// Same effect as testing.T.Error
// Generic definition for compatibility with test.T interface
// Implements test.T
func (o *TLogger) Error(stringThenKeysAndValues ...interface{}) {
	// Using o.error to have consistent call depth for Error, FatalIfErr, Info, etc
	o.error(o.errorWithRuntimeCheck(stringThenKeysAndValues...))
	o.fail()
}

// Fatal is essentially a .V(errorLevel).Info() followed by failing and immediately stopping the test.
// Intended usage is Fatal(msg string, key-value alternating arguments)
// Same effect as testing.T.Fatal
// Generic definition for compatibility with test.TLegacy interface
// Implements test.TLegacy
func (o *TLogger) Fatal(stringThenKeysAndValues ...interface{}) {
	o.error(o.errorWithRuntimeCheck(stringThenKeysAndValues...))
	o.failNow()
}

func (o *TLogger) fail() {
	if o.t != nil && !o.dontFail {
		o.t.Fail()
	}
}

func (o *TLogger) failNow() {
	if o.t != nil && !o.dontFail {
		o.t.FailNow()
	}
}

func validateKeysAndValues(keysAndValues ...interface{}) bool {
	length := len(keysAndValues)
	for i := 0; i < length; {
		_, isField := keysAndValues[i].(zapcore.Field)
		_, isString := keysAndValues[i].(string)
		if isField {
			i += 1
		} else if isString {
			if i == length-1 {
				return false
			}
			i += 2
		} else {
			return false
		}
	}
	return true
}

func (o *TLogger) interfacesToFields(things ...interface{}) []interface{} {
	o.V(5).Info("DEPRECATED Error/Fatal usage", zap.Stack("callstack"))
	fields := make([]interface{}, 2*len(things))
	for i, d := range things {
		fields[i*2] = fmt.Sprintf("arg %d", i)
		fields[i*2+1] = d
	}
	return fields
}

func (o *TLogger) errorWithRuntimeCheck(stringThenKeysAndValues ...interface{}) (error, string, []interface{}) {
	if len(stringThenKeysAndValues) == 0 {
		return nil, "", nil
	}
	s, isString := stringThenKeysAndValues[0].(string)
	if isString {
		// Desired case (hopefully)
		remainder := stringThenKeysAndValues[1:]
		if !validateKeysAndValues(remainder...) {
			remainder = o.interfacesToFields(remainder...)
		}
		return nil, s, remainder
	}
	e, isError := stringThenKeysAndValues[0].(error)
	if isError && len(stringThenKeysAndValues) == 1 {
		return e, "", nil
	}
	return nil, "unstructured error", o.interfacesToFields(stringThenKeysAndValues...)
}

// Cleanup registers a cleanup callback.
func (o *TLogger) Cleanup(c func()) {
	o.t.Cleanup(c)
}

// Run a subtest. Just like testing.T.Run but creates a TLogger.
func (o *TLogger) Run(name string, f func(t *TLogger)) {
	tfunc := func(ts *testing.T) {
		tl, cancel := newTLogger(ts, o.level, o.dontFail)
		defer cancel()
		f(tl)
	}
	o.t.Run(name, tfunc)
}

// Name is just like testing.T.Name()
// Implements test.T
func (o *TLogger) Name() string {
	return o.t.Name()
}

// Helper cannot work as an indirect call, so just do nothing :(
// Implements test.T
func (o *TLogger) Helper() {
}

// SkipNow immediately stops test execution
// Implements test.T
func (o *TLogger) SkipNow() {
	o.t.SkipNow()
}

// Log is deprecated: only existing for test.T compatibility
// Please use leveled logging via .V().Info()
// Will panic if given data incompatible with Info() function
// Implements test.T
func (o *TLogger) Log(args ...interface{}) {
	// This is complicated to ensure exactly 2 levels of indirection
	i := o.V(2)
	iL, ok := i.(*infoLogger)
	if ok {
		iL.indirectWrite(args[0].(string), args[1:]...)
	}
}

// Parallel allows tests or subtests to run in parallel
// Just calls the testing.T.Parallel() under the hood
func (o *TLogger) Parallel() {
	o.t.Parallel()
}

// Logf is deprecated: only existing for test.TLegacy compatibility
// Please use leveled logging via .V().Info()
// Implements test.TLegacy
func (o *TLogger) Logf(fmtS string, args ...interface{}) {
	// This is complicated to ensure exactly 2 levels of indirection
	iL, ok := o.V(2).(*infoLogger)
	if ok {
		iL.indirectWrite(fmt.Sprintf(fmtS, args...))
	}
}

func (o *TLogger) error(err error, msg string, keysAndValues []interface{}) {
	var newKAV []interface{}
	var serr StructuredError
	if errors.As(err, &serr) {
		serr.DisableValuePrinting()
		defer serr.EnableValuePrinting()
		newLen := len(keysAndValues) + len(serr.GetValues())
		newKAV = make([]interface{}, 0, newLen+2)
		newKAV = append(newKAV, keysAndValues...)
		newKAV = append(newKAV, serr.GetValues()...)
	}
	if err != nil {
		if msg == "" { // This is used if just the error is given to .Error() or .Fatal()
			msg = err.Error()
		} else {
			if newKAV == nil {
				newKAV = make([]interface{}, 0, len(keysAndValues)+1)
				newKAV = append(newKAV, keysAndValues...)
			}
			newKAV = append(newKAV, zap.Error(err))
		}
	}
	if newKAV != nil {
		keysAndValues = newKAV
	}
	if checkedEntry := o.l.Check(zap.ErrorLevel, msg); checkedEntry != nil {
		checkedEntry.Write(o.handleFields(keysAndValues)...)
	}
}

// Creation and Teardown

// Create a TLogger object using the global Zap logger and the current testing.T
// `defer` a call to second return value immediately after.
func NewTLogger(t *testing.T) (*TLogger, func()) {
	return newTLogger(t, verbosity, false)
}

func newTLogger(t *testing.T, verbosity int, dontFail bool) (*TLogger, func()) {
	testOptions := []zap.Option{
		zap.AddCaller(),
		zap.AddCallerSkip(2),
		zap.Development(),
	}
	writer := newTestingWriter(t)
	// Based off zap.NewDevelopmentEncoderConfig()
	cfg := zapcore.EncoderConfig{
		// Wanted keys can be anything except the empty string.
		TimeKey:        "",
		LevelKey:       "",
		NameKey:        "",
		CallerKey:      "C",
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	core := zapcore.NewCore(
		NewSpewEncoder(cfg),
		writer,
		zapcore.DebugLevel,
	)
	if zapCore != nil {
		core = zapcore.NewTee(
			zapCore,
			core,
			// TODO(coryrc): Open new file (maybe creating JUnit!?) with test output?
		)
	}
	log := zap.New(core, testOptions...).Named(t.Name())
	tlogger := TLogger{
		l:        log,
		level:    verbosity,
		t:        t,
		errs:     make(map[string][]interface{}),
		dontFail: dontFail,
	}
	return &tlogger, func() {
		tlogger.handleCollectedErrors()
		// Sometimes goroutines exist after a test and they cause panics if they attempt to call t.Log().
		// Prevent this panic by disabling writes to the testing.T (we'll still get them everywhere else).
		writer.Disable()
	}
}

func (o *TLogger) cloneWithNewLogger(l *zap.Logger) *TLogger {
	t := TLogger{
		l:        l,
		level:    o.level,
		t:        o.t,
		errs:     o.errs,
		dontFail: o.dontFail,
	}
	return &t
}

// Collect allows you to commingle multiple validations during one test execution.
// Under the hood, it creates a sub-test during cleanup and iterates through the collected values, printing them.
// If any are errors, it fails the subtest.
// Currently experimental and likely to be removed
func (o *TLogger) Collect(key string, value interface{}) {
	list, has_key := o.errs[key]
	if has_key {
		list = append(list, value)
	} else {
		list = make([]interface{}, 1)
		list[0] = value
	}
	o.errs[key] = list
}

func (o *TLogger) handleCollectedErrors() {
	for name, list := range o.errs {
		o.Run(name, func(t *TLogger) {
			for _, item := range list {
				_, isError := item.(error)
				if isError {
					t.Error(item)
				} else {
					t.V(3).Info(spewConfig.Sprint(item))
				}
			}
		})
	}
}
