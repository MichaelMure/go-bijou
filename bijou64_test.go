package bijou_test

import (
	"bytes"
	"io"
	"math/rand/v2"
	"testing"

	"github.com/MichaelMure/go-bijou"
)

// testVectors are the positive test vectors from the bijou spec:
// https://github.com/inkandswitch/bijou/blob/main/bijou/SPEC.md#test-vectors
var testVectors = []struct {
	value uint64
	enc   []byte
}{
	{0, []byte{0x00}},
	{1, []byte{0x01}},
	{42, []byte{0x2A}},
	{247, []byte{0xF7}},
	{248, []byte{0xF8, 0x00}},
	{300, []byte{0xF8, 0x34}},
	{503, []byte{0xF8, 0xFF}},
	{504, []byte{0xF9, 0x00, 0x00}},
	{1000, []byte{0xF9, 0x01, 0xF0}},
	{65535, []byte{0xF9, 0xFE, 0x07}},
	{66039, []byte{0xF9, 0xFF, 0xFF}},
	{66040, []byte{0xFA, 0x00, 0x00, 0x00}},
	{67000, []byte{0xFA, 0x00, 0x03, 0xC0}},
	{16843255, []byte{0xFA, 0xFF, 0xFF, 0xFF}},
	{16843256, []byte{0xFB, 0x00, 0x00, 0x00, 0x00}},
	{4311810551, []byte{0xFB, 0xFF, 0xFF, 0xFF, 0xFF}},
	{72340172838076920, []byte{0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
	{18446744073709551615, []byte{0xFF, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0x07}},
}

func TestEncodeVectors(t *testing.T) {
	for _, v := range testVectors {
		got := bijou.EncodeU64(v.value)
		if !bytes.Equal(got, v.enc) {
			t.Errorf("EncodeU64(%d) = %X, want %X", v.value, got, v.enc)
		}
	}
}

func TestDecodeVectors(t *testing.T) {
	for _, v := range testVectors {
		r := bytes.NewReader(v.enc)
		got, err := bijou.DecodeU64(r)
		if err != nil {
			t.Errorf("DecodeU64(%X) error: %v", v.enc, err)
			continue
		}
		if got != v.value {
			t.Errorf("DecodeU64(%X) = %d, want %d", v.enc, got, v.value)
		}
	}
}

func TestDecodeBytesVectors(t *testing.T) {
	for _, v := range testVectors {
		got, n, err := bijou.DecodeBytes(v.enc)
		if err != nil {
			t.Errorf("DecodeBytes(%X) error: %v", v.enc, err)
			continue
		}
		if got != v.value {
			t.Errorf("DecodeBytes(%X) = %d, want %d", v.enc, got, v.value)
		}
		if n != len(v.enc) {
			t.Errorf("DecodeBytes(%X) consumed %d bytes, want %d", v.enc, n, len(v.enc))
		}
	}
}

func TestRoundTrip(t *testing.T) {
	for _, v := range testVectors {
		buf := bijou.AppendU64(nil, v.value)
		r := bytes.NewReader(buf)
		got, err := bijou.DecodeU64(r)
		if err != nil {
			t.Errorf("round-trip %d: %v", v.value, err)
			continue
		}
		if got != v.value {
			t.Errorf("round-trip %d: got %d", v.value, got)
		}
	}
}

// TestDecodeExactConsumption verifies that decoding reads exactly the bytes
// for one value and leaves any following bytes untouched — critical when many
// values are packed back-to-back in a single buffer.
func TestDecodeExactConsumption(t *testing.T) {
	var packed []byte
	for _, v := range testVectors {
		packed = append(packed, v.enc...)
	}
	r := bytes.NewReader(packed)
	for _, v := range testVectors {
		got, err := bijou.DecodeU64(r)
		if err != nil {
			t.Fatalf("value %d: unexpected error %v", v.value, err)
		}
		if got != v.value {
			t.Fatalf("value %d: got %d", v.value, got)
		}
	}
	if r.Len() != 0 {
		t.Fatalf("%d unexpected bytes remaining in buffer", r.Len())
	}
}

func TestDecodeBytesExactConsumption(t *testing.T) {
	var packed []byte
	for _, v := range testVectors {
		packed = append(packed, v.enc...)
	}
	pos := 0
	for _, v := range testVectors {
		got, n, err := bijou.DecodeBytes(packed[pos:])
		if err != nil {
			t.Fatalf("value %d: unexpected error %v", v.value, err)
		}
		if got != v.value {
			t.Fatalf("value %d: got %d", v.value, got)
		}
		pos += n
	}
	if pos != len(packed) {
		t.Fatalf("%d unexpected bytes remaining", len(packed)-pos)
	}
}

func TestDecodeBytesErrorVectors(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		_, _, err := bijou.DecodeBytes(nil)
		if err != bijou.ErrBufferTooShort {
			t.Errorf("empty input: got %v, want ErrBufferTooShort", err)
		}
	})
	t.Run("truncated", func(t *testing.T) {
		_, _, err := bijou.DecodeBytes([]byte{0xF9, 0x00})
		if err != bijou.ErrBufferTooShort {
			t.Errorf("truncated: got %v, want ErrBufferTooShort", err)
		}
	})
	t.Run("overflow", func(t *testing.T) {
		_, _, err := bijou.DecodeBytes([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
		if err != bijou.ErrOverflow {
			t.Errorf("overflow: got %v, want ErrOverflow", err)
		}
	})
}

func TestErrorVectors(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		_, err := bijou.DecodeU64(bytes.NewReader(nil))
		if err != io.EOF {
			t.Errorf("empty input: got %v, want io.EOF", err)
		}
	})
	t.Run("truncated", func(t *testing.T) {
		_, err := bijou.DecodeU64(bytes.NewReader([]byte{0xF9, 0x00}))
		if err != bijou.ErrBufferTooShort {
			t.Errorf("truncated: got %v, want ErrBufferTooShort", err)
		}
	})
	t.Run("overflow", func(t *testing.T) {
		_, err := bijou.DecodeU64(bytes.NewReader([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}))
		if err != bijou.ErrOverflow {
			t.Errorf("overflow: got %v, want ErrOverflow", err)
		}
	})
}

// FuzzRoundTrip verifies that any valid encoding decodes back to the original
// value, and that the decoder never panics on arbitrary input.
func FuzzRoundTrip(f *testing.F) {
	for _, v := range testVectors {
		f.Add(v.enc)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		v, err := bijou.DecodeU64(r)
		if err != nil {
			return
		}
		got := bijou.EncodeU64(v)
		if !bytes.Equal(got, data[:len(data)-r.Len()]) {
			t.Fatalf("round-trip failed: decoded %d, re-encoded to %X, original %X",
				v, got, data[:len(data)-r.Len()])
		}
	})
}

// ---------------------------------------------------------------------------
// Benchmark distributions — mirror the Rust shootout benchmarks exactly:
// same ranges, same batch size (4096), same fixed seed.
// https://github.com/inkandswitch/bijou/blob/main/bijou/benches/shootout.rs
// ---------------------------------------------------------------------------

const benchBatch = 4096
const benchSeed = 0xBEEFCAFEDEADF00D

func makeRNG() *rand.Rand {
	return rand.New(rand.NewPCG(benchSeed, 0))
}

func tinyValues() []uint64 {
	rng := makeRNG()
	vals := make([]uint64, benchBatch)
	for i := range vals {
		vals[i] = rng.Uint64N(248) // 0–247
	}
	return vals
}

func smallValues() []uint64 {
	rng := makeRNG()
	vals := make([]uint64, benchBatch)
	for i := range vals {
		vals[i] = 248 + rng.Uint64N(65535-248+1) // 248–65535
	}
	return vals
}

func mediumValues() []uint64 {
	rng := makeRNG()
	vals := make([]uint64, benchBatch)
	for i := range vals {
		vals[i] = 65536 + rng.Uint64N(uint64(^uint32(0))-65536+1) // 65536–4294967295
	}
	return vals
}

func largeValues() []uint64 {
	rng := makeRNG()
	vals := make([]uint64, benchBatch)
	for i := range vals {
		vals[i] = uint64(^uint32(0)) + 1 + rng.Uint64N(^uint64(0)-(uint64(^uint32(0))+1)+1) // >4G
	}
	return vals
}

func uniformValues() []uint64 {
	rng := makeRNG()
	vals := make([]uint64, benchBatch)
	for i := range vals {
		vals[i] = rng.Uint64()
	}
	return vals
}

func boundaryValues() []uint64 {
	boundaries := []uint64{
		0, 247, 248, 503, 504,
		66_039, 66_040, 16_843_255, 16_843_256,
		4_311_810_551, 4_311_810_552,
		1_103_823_438_327, 1_103_823_438_328,
		282_578_800_148_983, 282_578_800_148_984,
		72_340_172_838_076_919, 72_340_172_838_076_920,
		^uint64(0),
	}
	vals := make([]uint64, benchBatch)
	for i := range vals {
		vals[i] = boundaries[i%len(boundaries)]
	}
	return vals
}

// preEncode returns a packed buffer and per-value byte offsets.
func preEncode(values []uint64) (buf []byte, offsets []int) {
	buf = make([]byte, 0, len(values)*5)
	offsets = make([]int, len(values))
	for i, v := range values {
		offsets[i] = len(buf)
		buf = bijou.AppendU64(buf, v)
	}
	return
}

// ---------------------------------------------------------------------------
// Encode benchmarks — one batch per b.Loop iteration; ns/op ÷ 4096 = ns/value
// ---------------------------------------------------------------------------

func benchEncode(b *testing.B, values []uint64) {
	buf := make([]byte, 0, len(values)*9)
	for b.Loop() {
		buf = buf[:0]
		for _, v := range values {
			buf = bijou.AppendU64(buf, v)
		}
	}
}

func BenchmarkEncodeTiny(b *testing.B)     { benchEncode(b, tinyValues()) }
func BenchmarkEncodeSmall(b *testing.B)    { benchEncode(b, smallValues()) }
func BenchmarkEncodeMedium(b *testing.B)   { benchEncode(b, mediumValues()) }
func BenchmarkEncodeLarge(b *testing.B)    { benchEncode(b, largeValues()) }
func BenchmarkEncodeBoundary(b *testing.B) { benchEncode(b, boundaryValues()) }
func BenchmarkEncodeUniform(b *testing.B)  { benchEncode(b, uniformValues()) }

// ---------------------------------------------------------------------------
// Decode benchmarks — random access via pre-computed offsets; matches
// Rust's bench_decode which also uses per-value offset tables.
// ---------------------------------------------------------------------------

func benchDecode(b *testing.B, values []uint64) {
	buf, offsets := preEncode(values)
	var sum uint64
	for b.Loop() {
		sum = 0
		for _, off := range offsets {
			v, _, _ := bijou.DecodeBytes(buf[off:])
			sum += v
		}
	}
	_ = sum
}

func BenchmarkDecodeTiny(b *testing.B)     { benchDecode(b, tinyValues()) }
func BenchmarkDecodeSmall(b *testing.B)    { benchDecode(b, smallValues()) }
func BenchmarkDecodeMedium(b *testing.B)   { benchDecode(b, mediumValues()) }
func BenchmarkDecodeLarge(b *testing.B)    { benchDecode(b, largeValues()) }
func BenchmarkDecodeBoundary(b *testing.B) { benchDecode(b, boundaryValues()) }
func BenchmarkDecodeUniform(b *testing.B)  { benchDecode(b, uniformValues()) }

// ---------------------------------------------------------------------------
// Stream decode benchmarks — sequential decode advancing through the buffer;
// matches Rust's bench_stream_decode.
// ---------------------------------------------------------------------------

func benchStreamDecode(b *testing.B, values []uint64) {
	buf, _ := preEncode(values)
	var sum uint64
	for b.Loop() {
		sum = 0
		pos := 0
		for pos < len(buf) {
			v, n, _ := bijou.DecodeBytes(buf[pos:])
			sum += v
			pos += n
		}
	}
	_ = sum
}

func BenchmarkStreamDecodeTiny(b *testing.B)     { benchStreamDecode(b, tinyValues()) }
func BenchmarkStreamDecodeSmall(b *testing.B)    { benchStreamDecode(b, smallValues()) }
func BenchmarkStreamDecodeMedium(b *testing.B)   { benchStreamDecode(b, mediumValues()) }
func BenchmarkStreamDecodeLarge(b *testing.B)    { benchStreamDecode(b, largeValues()) }
func BenchmarkStreamDecodeBoundary(b *testing.B) { benchStreamDecode(b, boundaryValues()) }
func BenchmarkStreamDecodeUniform(b *testing.B)  { benchStreamDecode(b, uniformValues()) }

// ---------------------------------------------------------------------------
// DecodeU64 stream benchmarks — same loop as benchStreamDecode but through
// the io.Reader interface rather than direct slice access.
// ---------------------------------------------------------------------------

func benchStreamDecodeU64(b *testing.B, values []uint64) {
	buf, _ := preEncode(values)
	r := bytes.NewReader(buf)
	var sum uint64
	for b.Loop() {
		r.Seek(0, 0)
		sum = 0
		for r.Len() > 0 {
			v, _ := bijou.DecodeU64(r)
			sum += v
		}
	}
	_ = sum
}

func BenchmarkStreamDecodeU64Tiny(b *testing.B)     { benchStreamDecodeU64(b, tinyValues()) }
func BenchmarkStreamDecodeU64Small(b *testing.B)    { benchStreamDecodeU64(b, smallValues()) }
func BenchmarkStreamDecodeU64Medium(b *testing.B)   { benchStreamDecodeU64(b, mediumValues()) }
func BenchmarkStreamDecodeU64Large(b *testing.B)    { benchStreamDecodeU64(b, largeValues()) }
func BenchmarkStreamDecodeU64Boundary(b *testing.B) { benchStreamDecodeU64(b, boundaryValues()) }
func BenchmarkStreamDecodeU64Uniform(b *testing.B)  { benchStreamDecodeU64(b, uniformValues()) }

// ---------------------------------------------------------------------------
// Encoded size benchmarks — matches Rust's bench_encoded_size.
// ---------------------------------------------------------------------------

func benchEncodedLen(b *testing.B, values []uint64) {
	var total int
	for b.Loop() {
		total = 0
		for _, v := range values {
			total += bijou.EncodedLen(v)
		}
	}
	_ = total
}

func BenchmarkEncodedLenTiny(b *testing.B)     { benchEncodedLen(b, tinyValues()) }
func BenchmarkEncodedLenSmall(b *testing.B)    { benchEncodedLen(b, smallValues()) }
func BenchmarkEncodedLenMedium(b *testing.B)   { benchEncodedLen(b, mediumValues()) }
func BenchmarkEncodedLenLarge(b *testing.B)    { benchEncodedLen(b, largeValues()) }
func BenchmarkEncodedLenBoundary(b *testing.B) { benchEncodedLen(b, boundaryValues()) }
func BenchmarkEncodedLenUniform(b *testing.B)  { benchEncodedLen(b, uniformValues()) }
