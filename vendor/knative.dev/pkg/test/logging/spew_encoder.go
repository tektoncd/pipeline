// Copyright 2020 The Knative Authors
// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package logging

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	. "go.uber.org/zap/zapcore"
)

var (
	_pool             = buffer.NewPool()
	_sliceEncoderPool = sync.Pool{
		New: func() interface{} {
			return &sliceArrayEncoder{elems: make([]interface{}, 0, 2)}
		},
	}
)

func init() {
	zap.RegisterEncoder("spew", func(encoderConfig EncoderConfig) (Encoder, error) {
		return NewSpewEncoder(encoderConfig), nil
	})
}

// NewSpewEncoder encodes logs using the spew library.
//
// The JSON encoder (also used by the console encoder) included in Zap can only print objects that
// can be serialized to JSON and doesn't print them in the most readable way. This spew encoder is
// designed to make human-readable log only and get the most information to the user on any data type.
//
// Code is mostly from console_encoder.go in zapcore.
func NewSpewEncoder(cfg EncoderConfig) *SpewEncoder {
	enc := SpewEncoder{}
	enc.MapObjectEncoder = NewMapObjectEncoder()
	enc.EncoderConfig = &cfg
	return &enc
}

// SpewEncoder implements zapcore.Encoder interface
type SpewEncoder struct {
	*MapObjectEncoder
	*EncoderConfig
}

// Implements zapcore.Encoder interface
func (enc *SpewEncoder) Clone() Encoder {
	n := NewSpewEncoder(*(enc.EncoderConfig))
	for k, v := range enc.Fields {
		n.Fields[k] = v
	}
	return n
}

func getSliceEncoder() *sliceArrayEncoder {
	return _sliceEncoderPool.Get().(*sliceArrayEncoder)
}

func putSliceEncoder(e *sliceArrayEncoder) {
	e.elems = e.elems[:0]
	_sliceEncoderPool.Put(e)
}

// Implements zapcore.Encoder interface.
func (enc *SpewEncoder) EncodeEntry(ent Entry, fields []Field) (*buffer.Buffer, error) {
	line := _pool.Get()

	// Could probably rewrite this portion and remove the copied
	// memory_encoder.go from this folder
	arr := getSliceEncoder()
	defer putSliceEncoder(arr)

	if enc.TimeKey != "" && enc.EncodeTime != nil {
		enc.EncodeTime(ent.Time, arr)
	}
	if enc.LevelKey != "" && enc.EncodeLevel != nil {
		enc.EncodeLevel(ent.Level, arr)
	}

	if ent.LoggerName != "" && enc.NameKey != "" {
		nameEncoder := enc.EncodeName

		if nameEncoder == nil {
			// Fall back to FullNameEncoder for backward compatibility.
			nameEncoder = FullNameEncoder
		}

		nameEncoder(ent.LoggerName, arr)
	}
	if ent.Caller.Defined && enc.CallerKey != "" && enc.EncodeCaller != nil {
		enc.EncodeCaller(ent.Caller, arr)
	}
	for i := range arr.elems {
		if i > 0 {
			line.AppendByte('\t')
		}
		fmt.Fprint(line, arr.elems[i])
	}

	// Add the message itself.
	if enc.MessageKey != "" {
		enc.addTabIfNecessary(line)
		line.AppendString(ent.Message)
	}

	// Add any structured context.
	enc.writeContext(line, fields)

	// If there's no stacktrace key, honor that; this allows users to force
	// single-line output.
	if ent.Stack != "" && enc.StacktraceKey != "" {
		line.AppendByte('\n')
		line.AppendString(ent.Stack)
	}

	if enc.LineEnding != "" {
		line.AppendString(enc.LineEnding)
	} else {
		line.AppendString(DefaultLineEnding)
	}
	return line, nil
}

func (enc *SpewEncoder) writeContext(line *buffer.Buffer, extra []Field) {
	if len(extra) == 0 && len(enc.Fields) == 0 {
		return
	}

	// This could probably be more efficient, but .AddTo() is convenient

	context := NewMapObjectEncoder()
	for k, v := range enc.Fields {
		context.Fields[k] = v
	}
	for i := range extra {
		extra[i].AddTo(context)
	}

	enc.addTabIfNecessary(line)
	line.AppendString("\nContext:\n")
	var keys []string
	for k := range context.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		line.AppendString(k)
		line.AppendString(": ")
		line.AppendString(stringify(context.Fields[k]))
		line.TrimNewline()
		line.AppendString("\n")
	}
}

func stringify(a interface{}) string {
	s, ok := a.(string)
	if !ok {
		s = strings.TrimSuffix(spewConfig.Sdump(a), "\n")
	}
	ret := strings.ReplaceAll(s, "\n", "\n ")
	hasNewlines := s != ret
	if hasNewlines {
		return "\n " + ret
	}
	return s
}

func (enc *SpewEncoder) addTabIfNecessary(line *buffer.Buffer) {
	if line.Len() > 0 {
		line.AppendByte('\t')
	}
}
