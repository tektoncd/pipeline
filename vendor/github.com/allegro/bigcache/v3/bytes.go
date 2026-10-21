//go:build !appengine
// +build !appengine

package bigcache

import (
	"unsafe"
)

func bytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
