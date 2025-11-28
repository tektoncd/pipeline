// Package safecast allows you to safely cast between numeric types in Go and return errors (or panic when using the
// Must* variants) when the cast would result in a loss of precision, range or sign.
package safecast

// Implementation: me (@ldemailly), idea: @ccoVeille - https://github.com/ccoVeille/go-safecast
// Started https://github.com/ldemailly/go-scratch/commits/main/safecast/safecast.go
// then moved here to fortio.org/safecast

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"unsafe"
)

type Integer interface {
	// Same as golang.org/x/contraints.Integer but without importing the whole thing for 1 line.
	~int | ~uint | ~int8 | ~uint8 | ~int16 | ~uint16 | ~int32 | ~uint32 | ~int64 | ~uint64 | ~uintptr
}

type Float interface {
	~float32 | ~float64 // Consider removing the ~ because of +Inf issue.
}

type Number interface {
	Integer | Float
}

var ErrOutOfRange = errors.New("out of range")

const (
	all63bits = uint64(math.MaxInt64)
	all31bits = uint64(math.MaxInt32)
)

func isFloat[Num Number](f Num) (isFloat bool) {
	switch reflect.TypeOf(f).Kind() { //nolint:exhaustive // only 2 we want to check
	case reflect.Float32, reflect.Float64:
		isFloat = true
	}
	return
}

// Convert converts a number from one type to another,
// returning an error if the conversion would result in a loss of precision,
// range or sign (overflow). In other words if the converted number is not
// equal to the original number.
// Use [Conv] instead if both of your types are Integer.
// Do not use for identity (same type in and out) but in particular this
// will error for Convert[uint64](uint64(math.MaxUint64)) or
// Convert[int64](int64(math.MaxInt64)) because it needs to
// when converting to any float. Note that +Inf will convert correctly (as in error
// only if going to an integer type) from a float64/float32 and not a ~float (it will
// error from say ~float32 to float64 while it shouldn't).
func Convert[NumOut Number, NumIn Number](orig NumIn) (converted NumOut, err error) {
	origPositive := (orig >= 0)
	// All bits set on uint64 or positive int63 are two of 4 special cases not detected by roundtrip (afaik).
	if origPositive && (uint64(orig)&all63bits == all63bits) && !isFloat(orig) {
		err = ErrOutOfRange
		return
	}
	converted = NumOut(orig)
	if origPositive != (converted >= 0) {
		err = ErrOutOfRange
		return
	}
	if NumIn(converted) != orig && ((converted == converted) || (orig == orig)) { //nolint:gocritic // NaN check
		err = ErrOutOfRange
		return
	}
	// And this are the other 2 weird case, maxint32 and maxuint32 (on armhf) conversion to float32.
	if origPositive && (uint64(orig)&all31bits == all31bits) && unsafe.Sizeof(converted) == 4 && !isFloat(orig) {
		err = ErrOutOfRange
	}
	return
}

// Conv is an integer only and simpler version of [Convert] without the
// weird issue with round trip to floating point.
// Unlike [Convert] it can be used for identity (same type in and out) even if that's
// of dubious value.
func Conv[NumOut Integer, NumIn Integer](orig NumIn) (converted NumOut, err error) {
	origPositive := (orig >= 0)
	converted = NumOut(orig)
	if origPositive != (converted >= 0) {
		err = ErrOutOfRange
		return
	}
	if NumIn(converted) != orig {
		err = ErrOutOfRange
	}
	return
}

// Same as Convert but panics if there is an error.
func MustConvert[NumOut Number, NumIn Number](orig NumIn) NumOut {
	converted, err := Convert[NumOut](orig)
	if err != nil {
		doPanic(err, orig, converted)
	}
	return converted
}

// MustConv is as [Conv] but panics if there is an error.
func MustConv[NumOut Integer, NumIn Integer](orig NumIn) NumOut {
	converted, err := Conv[NumOut](orig)
	if err != nil {
		doPanic(err, orig, converted)
	}
	return converted
}

// Converts a float to an integer by truncating the fractional part.
// Returns an error if the conversion would result in a loss of precision.
func Truncate[NumOut Number, NumIn Float](orig NumIn) (converted NumOut, err error) {
	return Convert[NumOut](math.Trunc(float64(orig)))
}

// Converts a float to an integer by rounding to the nearest integer.
// Returns an error if the conversion would result in a loss of precision.
func Round[NumOut Number, NumIn Float](orig NumIn) (converted NumOut, err error) {
	return Convert[NumOut](math.Round(float64(orig)))
}

// Same as Truncate but panics if there is an error.
func MustTruncate[NumOut Number, NumIn Float](orig NumIn) NumOut {
	converted, err := Truncate[NumOut, NumIn](orig)
	if err != nil {
		doPanic(err, orig, converted)
	}
	return converted
}

// Same as Round but panics if there is an error.
func MustRound[NumOut Number, NumIn Float](orig NumIn) NumOut {
	converted, err := Round[NumOut, NumIn](orig)
	if err != nil {
		doPanic(err, orig, converted)
	}
	return converted
}

func doPanic[NumOut Number, NumIn Number](err error, orig NumIn, converted NumOut) {
	panic(fmt.Sprintf("safecast: %v for %v (%T) to %T", err, orig, orig, converted))
}
