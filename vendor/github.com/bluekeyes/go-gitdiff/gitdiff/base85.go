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
// alphabet defined by base85.c in the Git source tree. src must contain at
// least len(dst) bytes of encoded data.
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

// base85Encode encodes src in Base85, writing the result to dst. It uses the
// alphabet defined by base85.c in the Git source tree.
func base85Encode(dst, src []byte) {
	var di, si int

	encode := func(v uint32) {
		dst[di+0] = b85Alpha[(v/(85*85*85*85))%85]
		dst[di+1] = b85Alpha[(v/(85*85*85))%85]
		dst[di+2] = b85Alpha[(v/(85*85))%85]
		dst[di+3] = b85Alpha[(v/85)%85]
		dst[di+4] = b85Alpha[v%85]
	}

	n := (len(src) / 4) * 4
	for si < n {
		encode(uint32(src[si+0])<<24 | uint32(src[si+1])<<16 | uint32(src[si+2])<<8 | uint32(src[si+3]))
		si += 4
		di += 5
	}

	var v uint32
	switch len(src) - si {
	case 3:
		v |= uint32(src[si+2]) << 8
		fallthrough
	case 2:
		v |= uint32(src[si+1]) << 16
		fallthrough
	case 1:
		v |= uint32(src[si+0]) << 24
		encode(v)
	}
}

// base85Len returns the length of n bytes of Base85 encoded data.
func base85Len(n int) int {
	return (n + 3) / 4 * 5
}
