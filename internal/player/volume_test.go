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
