package player

import (
	"encoding/binary"
	"io"
)

type volumeReader struct {
	src    io.Reader
	factor func() float64
}

func newVolumeReader(src io.Reader, factor func() float64) *volumeReader {
	return &volumeReader{src: src, factor: factor}
}

func (r *volumeReader) Read(p []byte) (int, error) {
	n, err := r.src.Read(p)
	if n == 0 {
		return n, err
	}
	v := r.factor()
	usable := n - (n % 2)
	for i := 0; i < usable; i += 2 {
		sample := int16(binary.LittleEndian.Uint16(p[i : i+2]))
		scaled := int16(float64(sample) * v)
		binary.LittleEndian.PutUint16(p[i:i+2], uint16(scaled))
	}
	return n, err
}
