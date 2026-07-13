package player

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

func samplesToBytes(samples []int16) []byte {
	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	return buf
}

func TestVolumeReader_ScalesSamplesByFactor(t *testing.T) {
	src := bytes.NewReader(samplesToBytes([]int16{1000, -1000, 2000}))
	vr := newVolumeReader(src, func() float64 { return 0.5 })

	out := make([]byte, 64)
	n, err := vr.Read(out)
	if err != nil && err != io.EOF {
		t.Fatalf("Read returned error: %v", err)
	}

	got := make([]int16, n/2)
	for i := range got {
		got[i] = int16(binary.LittleEndian.Uint16(out[i*2 : i*2+2]))
	}
	want := []int16{500, -500, 1000}
	if len(got) != len(want) {
		t.Fatalf("Read() produced %d samples, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestVolumeReader_ZeroFactorSilences(t *testing.T) {
	src := bytes.NewReader(samplesToBytes([]int16{12345, -12345}))
	vr := newVolumeReader(src, func() float64 { return 0 })

	out := make([]byte, 64)
	n, _ := vr.Read(out)

	for i := 0; i < n; i++ {
		if out[i] != 0 {
			t.Fatalf("byte[%d] = %d, want 0 when muted", i, out[i])
		}
	}
}

// chunkedReader deliberately returns data in fixed-size (potentially odd
// relative to sample boundaries) chunks across multiple Read calls, unlike
// bytes.Reader which returns everything available in a single Read. This
// lets tests force an odd-length Read followed by a subsequent Read, to
// exercise carry-over logic across the volumeReader.Read call boundary.
type chunkedReader struct {
	data  []byte
	pos   int
	chunk int
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := r.chunk
	if n > len(p) {
		n = len(p)
	}
	if r.pos+n > len(r.data) {
		n = len(r.data) - r.pos
	}
	copy(p, r.data[r.pos:r.pos+n])
	r.pos += n
	return n, nil
}

func TestVolumeReader_CarriesOddByteAcrossReads(t *testing.T) {
	data := samplesToBytes([]int16{1000, -1000, 2000})
	src := &chunkedReader{data: data, chunk: 3}
	vr := newVolumeReader(src, func() float64 { return 0.5 })

	out1 := make([]byte, 64)
	n1, err := vr.Read(out1)
	if err != nil && err != io.EOF {
		t.Fatalf("first Read returned error: %v", err)
	}

	out2 := make([]byte, 64)
	n2, err := vr.Read(out2)
	if err != nil && err != io.EOF {
		t.Fatalf("second Read returned error: %v", err)
	}

	got := append(append([]byte{}, out1[:n1]...), out2[:n2]...)
	want := samplesToBytes([]int16{500, -500, 1000})

	if len(got) != len(want) {
		t.Fatalf("reconstructed %d bytes, want %d (out1=%d bytes, out2=%d bytes)", len(got), len(want), n1, n2)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte[%d] = %d, want %d (got=%v, want=%v)", i, got[i], want[i], got, want)
		}
	}
}
