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
)

type Integer interface {
	// Same as golang.org/x/contraints.Integer but without importing the whole thing for 1 line.
	~int | ~uint | ~int8 | ~uint8 | ~int16 | ~uint16 | ~int32 | ~uint32 | ~int64 | ~uint64 | ~uintptr
}

type Float interface {
	~float32 | ~float64
}

type Number interface {
	Integer | Float
}

var ErrOutOfRange = errors.New("out of range")

const all64bitsOne = ^uint64(0) // same as uint64(math.MaxUint64)

// Convert converts a number from one type to another,
// returning an error if the conversion would result in a loss of precision,
// range or sign (overflow). In other words if the converted number is not
// equal to the original number.
// Do not use for identity (same type in and out) but in particular this
// will error for Convert[uint64](uint64(math.MaxUint64)) because it needs to
// when converting to any float.
func Convert[NumOut Number, NumIn Number](orig NumIn) (converted NumOut, err error) {
	origPositive := orig > 0
	// all bits set on uint64 is the only special case not detected by roundtrip (afaik).
	if origPositive && (uint64(orig) == all64bitsOne) {
		err = ErrOutOfRange
		return
	}
	converted = NumOut(orig)
	if origPositive != (converted > 0) {
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
	converted, err := Convert[NumOut, NumIn](orig)
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
