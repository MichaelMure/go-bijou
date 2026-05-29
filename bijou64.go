// Package bijou implements bijou64 encoding for unsigned 64-bit integers.
//
// bijou64 is a bijective variable-length encoding that uses 1–9 bytes.
// Every value has exactly one encoding, enforced structurally by per-tier
// offset arithmetic rather than runtime canonicality checks.
//
// See https://github.com/inkandswitch/bijou/blob/main/bijou64/SPEC.md
package bijou

import (
	"errors"
	"io"
)

var (
	ErrBufferTooShort = errors.New("bijou64: buffer too short")
	ErrOverflow       = errors.New("bijou64: integer overflow")
)

// Per-tier offsets: the first value encoded at each tier.
// Recurrence: offset[0]=0, offset[n] = offset[n-1] + 256^(n-1) for n≥1.
// The staircase pattern in hex (each tier prepends one 0x01 byte before 0xF8)
// reflects the geometric recurrence directly.
const (
	offset1 uint64 = 0xF8
	offset2 uint64 = 0x1F8
	offset3 uint64 = 0x101F8
	offset4 uint64 = 0x10101F8
	offset5 uint64 = 0x1010101F8
	offset6 uint64 = 0x101010101F8
	offset7 uint64 = 0x10101010101F8
	offset8 uint64 = 0x1010101010101F8
)

// offsets[tier] is the first value encoded at that tier (used for decode).
var offsets = [9]uint64{0, offset1, offset2, offset3, offset4, offset5, offset6, offset7, offset8}

// AppendU64 appends the bijou64 encoding of v to buf and returns the result.
func AppendU64(buf []byte, v uint64) []byte {
	switch {
	case v < offset1:
		return append(buf, byte(v))
	case v < offset2:
		return append(buf, 0xF8, byte(v-offset1))
	case v < offset3:
		p := v - offset2
		return append(buf, 0xF9, byte(p>>8), byte(p))
	case v < offset4:
		p := v - offset3
		return append(buf, 0xFA, byte(p>>16), byte(p>>8), byte(p))
	case v < offset5:
		p := v - offset4
		return append(buf, 0xFB, byte(p>>24), byte(p>>16), byte(p>>8), byte(p))
	case v < offset6:
		p := v - offset5
		return append(buf, 0xFC, byte(p>>32), byte(p>>24), byte(p>>16), byte(p>>8), byte(p))
	case v < offset7:
		p := v - offset6
		return append(buf, 0xFD, byte(p>>40), byte(p>>32), byte(p>>24), byte(p>>16), byte(p>>8), byte(p))
	case v < offset8:
		p := v - offset7
		return append(buf, 0xFE, byte(p>>48), byte(p>>40), byte(p>>32), byte(p>>24), byte(p>>16), byte(p>>8), byte(p))
	default:
		p := v - offset8
		return append(buf, 0xFF, byte(p>>56), byte(p>>48), byte(p>>40), byte(p>>32), byte(p>>24), byte(p>>16), byte(p>>8), byte(p))
	}
}

// EncodeU64 returns the bijou64 encoding of v as a new byte slice.
func EncodeU64(v uint64) []byte {
	return AppendU64(make([]byte, 0, 9), v)
}

// EncodedLen returns the number of bytes required to encode v.
func EncodedLen(v uint64) int {
	switch {
	case v < offset1:
		return 1
	case v < offset2:
		return 2
	case v < offset3:
		return 3
	case v < offset4:
		return 4
	case v < offset5:
		return 5
	case v < offset6:
		return 6
	case v < offset7:
		return 7
	case v < offset8:
		return 8
	default:
		return 9
	}
}

// DecodeBytes decodes a bijou64-encoded value from buf.
// Returns the value, the number of bytes consumed, and any error.
// Prefer this over DecodeU64 when decoding from a []byte — it avoids
// per-byte interface dispatch and uses direct indexed reads.
func DecodeBytes(buf []byte) (uint64, int, error) {
	if len(buf) == 0 {
		return 0, 0, ErrBufferTooShort
	}
	tag := buf[0]
	if tag < 0xF8 {
		return uint64(tag), 1, nil
	}

	tier := int(tag - 0xF7) // 0xF8-->1, 0xF9-->2, ..., 0xFF-->8
	if len(buf) < 1+tier {
		return 0, 0, ErrBufferTooShort
	}

	// Unrolled big-endian payload reads; bounds already checked above.
	var payload uint64
	switch tier {
	case 1:
		payload = uint64(buf[1])
	case 2:
		payload = uint64(buf[1])<<8 | uint64(buf[2])
	case 3:
		payload = uint64(buf[1])<<16 | uint64(buf[2])<<8 | uint64(buf[3])
	case 4:
		payload = uint64(buf[1])<<24 | uint64(buf[2])<<16 | uint64(buf[3])<<8 | uint64(buf[4])
	case 5:
		payload = uint64(buf[1])<<32 | uint64(buf[2])<<24 | uint64(buf[3])<<16 | uint64(buf[4])<<8 | uint64(buf[5])
	case 6:
		payload = uint64(buf[1])<<40 | uint64(buf[2])<<32 | uint64(buf[3])<<24 | uint64(buf[4])<<16 | uint64(buf[5])<<8 | uint64(buf[6])
	case 7:
		payload = uint64(buf[1])<<48 | uint64(buf[2])<<40 | uint64(buf[3])<<32 | uint64(buf[4])<<24 | uint64(buf[5])<<16 | uint64(buf[6])<<8 | uint64(buf[7])
	case 8:
		payload = uint64(buf[1])<<56 | uint64(buf[2])<<48 | uint64(buf[3])<<40 | uint64(buf[4])<<32 | uint64(buf[5])<<24 | uint64(buf[6])<<16 | uint64(buf[7])<<8 | uint64(buf[8])
	}

	v := offsets[tier] + payload
	if tier == 8 && v < offset8 {
		return 0, 0, ErrOverflow
	}
	return v, 1 + tier, nil
}

// DecodeU64 reads a bijou64-encoded value from r and returns it.
// Returns io.EOF if r is empty, ErrBufferTooShort if the payload is truncated,
// or ErrOverflow if the tier-8 value exceeds math.MaxUint64.
func DecodeU64(r io.ByteReader) (uint64, error) {
	tag, err := r.ReadByte()
	if err != nil {
		return 0, err // propagate io.EOF on empty input
	}
	if tag < 0xF8 {
		return uint64(tag), nil
	}

	tier := int(tag - 0xF7) // 0xF8-->1, 0xF9-->2, ..., 0xFF-->8
	var payload uint64
	for i := 0; i < tier; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, ErrBufferTooShort
		}
		payload = (payload << 8) | uint64(b)
	}

	v := offsets[tier] + payload
	if tier == 8 && v < offset8 {
		return 0, ErrOverflow
	}
	return v, nil
}
