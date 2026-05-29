package bijou64_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/MichaelMure/go-bijou64"
)

// testVectors are the positive test vectors from the bijou64 spec:
// https://github.com/inkandswitch/bijou/blob/main/bijou64/SPEC.md#test-vectors
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
		got := bijou64.EncodeU64(v.value)
		if !bytes.Equal(got, v.enc) {
			t.Errorf("EncodeU64(%d) = %X, want %X", v.value, got, v.enc)
		}
	}
}

func TestDecodeVectors(t *testing.T) {
	for _, v := range testVectors {
		r := bytes.NewReader(v.enc)
		got, err := bijou64.DecodeU64(r)
		if err != nil {
			t.Errorf("DecodeU64(%X) error: %v", v.enc, err)
			continue
		}
		if got != v.value {
			t.Errorf("DecodeU64(%X) = %d, want %d", v.enc, got, v.value)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	for _, v := range testVectors {
		buf := bijou64.AppendU64(nil, v.value)
		r := bytes.NewReader(buf)
		got, err := bijou64.DecodeU64(r)
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
		got, err := bijou64.DecodeU64(r)
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

func TestErrorVectors(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		_, err := bijou64.DecodeU64(bytes.NewReader(nil))
		if err != io.EOF {
			t.Errorf("empty input: got %v, want io.EOF", err)
		}
	})
	t.Run("truncated", func(t *testing.T) {
		_, err := bijou64.DecodeU64(bytes.NewReader([]byte{0xF9, 0x00}))
		if err != bijou64.ErrBufferTooShort {
			t.Errorf("truncated: got %v, want ErrBufferTooShort", err)
		}
	})
	t.Run("overflow", func(t *testing.T) {
		_, err := bijou64.DecodeU64(bytes.NewReader([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}))
		if err != bijou64.ErrOverflow {
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
		v, err := bijou64.DecodeU64(r)
		if err != nil {
			return
		}
		got := bijou64.EncodeU64(v)
		if !bytes.Equal(got, data[:len(data)-r.Len()]) {
			t.Fatalf("round-trip failed: decoded %d, re-encoded to %X, original %X",
				v, got, data[:len(data)-r.Len()])
		}
	})
}

// packedBench is a mixed-tier buffer used by BenchmarkDecodePacked.
var packedBench = func() []byte {
	vals := []uint64{0, 42, 247, 300, 67000, 4311810551, 72340172838076920, 18446744073709551615}
	var b []byte
	for _, v := range vals {
		b = bijou64.AppendU64(b, v)
	}
	return b
}()

var packedBenchCount = 8

func BenchmarkEncodeSmall(b *testing.B) {
	buf := make([]byte, 0, 9)
	for b.Loop() {
		buf = bijou64.AppendU64(buf[:0], 42)
	}
}

func BenchmarkEncodeMid(b *testing.B) {
	buf := make([]byte, 0, 9)
	for b.Loop() {
		buf = bijou64.AppendU64(buf[:0], 67000)
	}
}

func BenchmarkEncodeLarge(b *testing.B) {
	buf := make([]byte, 0, 9)
	for b.Loop() {
		buf = bijou64.AppendU64(buf[:0], 18446744073709551615)
	}
}

func BenchmarkDecodeSmall(b *testing.B) {
	r := bytes.NewReader([]byte{0x2A})
	for b.Loop() {
		r.Seek(0, io.SeekStart)
		bijou64.DecodeU64(r)
	}
}

func BenchmarkDecodeMid(b *testing.B) {
	r := bytes.NewReader([]byte{0xFA, 0x00, 0x03, 0xC0})
	for b.Loop() {
		r.Seek(0, io.SeekStart)
		bijou64.DecodeU64(r)
	}
}

func BenchmarkDecodeLarge(b *testing.B) {
	r := bytes.NewReader([]byte{0xFF, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0x07})
	for b.Loop() {
		r.Seek(0, io.SeekStart)
		bijou64.DecodeU64(r)
	}
}

// BenchmarkDecodePacked measures the realistic case: many values packed
// consecutively in a single buffer, decoded sequentially.
func BenchmarkDecodePacked(b *testing.B) {
	r := bytes.NewReader(packedBench)
	for b.Loop() {
		r.Seek(0, io.SeekStart)
		for range packedBenchCount {
			bijou64.DecodeU64(r)
		}
	}
}
