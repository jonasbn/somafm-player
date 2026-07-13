package player

import (
	"encoding/binary"
	"io"
)

type volumeReader struct {
	src      io.Reader
	factor   func() float64
	carry    byte
	hasCarry bool
}

func newVolumeReader(src io.Reader, factor func() float64) *volumeReader {
	return &volumeReader{src: src, factor: factor}
}

func (r *volumeReader) Read(p []byte) (int, error) {
	offset := 0
	if r.hasCarry {
		if len(p) == 0 {
			return 0, nil
		}
		p[0] = r.carry
		offset = 1
		r.hasCarry = false
	}

	n, err := r.src.Read(p[offset:])
	total := offset + n
	usable := total - (total % 2)

	v := r.factor()
	for i := 0; i < usable; i += 2 {
		sample := int16(binary.LittleEndian.Uint16(p[i : i+2]))
		scaled := int16(float64(sample) * v)
		binary.LittleEndian.PutUint16(p[i:i+2], uint16(scaled))
	}

	if total > usable {
		r.carry = p[usable]
		r.hasCarry = true
	}

	return usable, err
}
