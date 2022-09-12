package gitdiff

import (
	"fmt"
)

var (
	b85Table map[byte]byte
	b85Alpha = []byte(
		"0123456789" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "abcdefghijklmnopqrstuvwxyz" + "!#$%&()*+-;<=>?@^_`{|}~",
	)
)

func init() {
	b85Table = make(map[byte]byte)
	for i, c := range b85Alpha {
		b85Table[c] = byte(i)
	}
}

// base85Decode decodes Base85-encoded data from src into dst. It uses the
// alphabet defined by base85.c in the Git source tree, which appears to be
// unique. src must contain at least len(dst) bytes of encoded data.
func base85Decode(dst, src []byte) error {
	var v uint32
	var n, ndst int
	for i, b := range src {
		if b, ok := b85Table[b]; ok {
			v = 85*v + uint32(b)
			n++
		} else {
			return fmt.Errorf("invalid base85 byte at index %d: 0x%X", i, src[i])
		}
		if n == 5 {
			rem := len(dst) - ndst
			for j := 0; j < 4 && j < rem; j++ {
				dst[ndst] = byte(v >> 24)
				ndst++
				v <<= 8
			}
			v = 0
			n = 0
		}
	}
	if n > 0 {
		return fmt.Errorf("base85 data terminated by underpadded sequence")
	}
	if ndst < len(dst) {
		return fmt.Errorf("base85 data underrun: %d < %d", ndst, len(dst))
	}
	return nil
}
