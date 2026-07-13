package spectrum

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestHannWindow_ZeroAtEdgesPeakAtCenter(t *testing.T) {
	w := hannWindow(8)
	if math.Abs(w[0]) > 1e-9 {
		t.Fatalf("w[0] = %v, want ~0", w[0])
	}
	if math.Abs(w[7]) > 1e-9 {
		t.Fatalf("w[7] = %v, want ~0", w[7])
	}
	if w[3] < 0.9 {
		t.Fatalf("w[3] (near center) = %v, want close to 1", w[3])
	}
}

func TestDownmixStereoInt16LE_AveragesChannelsAndNormalizes(t *testing.T) {
	pcm := make([]byte, 8)
	binary.LittleEndian.PutUint16(pcm[0:], uint16(int16(1000)))
	negVal := int16(-1000)
	binary.LittleEndian.PutUint16(pcm[2:], uint16(negVal))
	binary.LittleEndian.PutUint16(pcm[4:], uint16(int16(20000)))
	binary.LittleEndian.PutUint16(pcm[6:], uint16(int16(20000)))

	got := downmixStereoInt16LE(pcm)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0] != 0 {
		t.Fatalf("got[0] = %v, want 0 (L=1000,R=-1000 average to 0)", got[0])
	}
	want1 := 20000.0 / 32768.0
	if math.Abs(got[1]-want1) > 1e-9 {
		t.Fatalf("got[1] = %v, want %v", got[1], want1)
	}
}

func TestDownmixStereoInt16LE_TruncatesPartialTrailingFrame(t *testing.T) {
	pcm := make([]byte, 6) // one full 4-byte frame + 2 leftover bytes
	got := downmixStereoInt16LE(pcm)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (trailing partial frame dropped)", len(got))
	}
}

func TestLogBandEdges_MonotonicFromLoToHi(t *testing.T) {
	edges := logBandEdges(4, 20, 20000)
	if len(edges) != 5 {
		t.Fatalf("len(edges) = %d, want 5", len(edges))
	}
	if math.Abs(edges[0]-20) > 1e-6 {
		t.Fatalf("edges[0] = %v, want 20", edges[0])
	}
	if math.Abs(edges[4]-20000) > 1e-6 {
		t.Fatalf("edges[4] = %v, want 20000", edges[4])
	}
	for i := 1; i < len(edges); i++ {
		if edges[i] <= edges[i-1] {
			t.Fatalf("edges not strictly increasing at index %d: %v <= %v", i, edges[i], edges[i-1])
		}
	}
}

func TestBandForFreq_AssignsToCorrectBucket(t *testing.T) {
	edges := []float64{10, 100, 1000, 10000}
	cases := []struct {
		freq float64
		want int
	}{
		{5, -1},
		{50, 0},
		{500, 1},
		{5000, 2},
		{10000, 2}, // top edge clamps into the last bucket
	}
	for _, c := range cases {
		if got := bandForFreq(c.freq, edges); got != c.want {
			t.Errorf("bandForFreq(%v) = %d, want %d", c.freq, got, c.want)
		}
	}
}

func TestBucketMagnitudes_GroupsIntoExpectedBandAndAverages(t *testing.T) {
	// Simulate a real-FFT coefficient slice for an 8-point window at
	// sampleRate=8000 (len = n/2+1 = 5, bin i => freq = i/8*8000 = i*1000Hz).
	// Put all the energy in bin 2 (2000Hz); everything else is silent.
	coeffs := make([]complex128, 5)
	coeffs[2] = complex(10, 0)

	bands := bucketMagnitudes(coeffs, 8000, 4)

	nonZero := 0
	peak := 0
	for i, v := range bands {
		if v != 0 {
			nonZero++
			peak = i
		}
	}
	if nonZero != 1 {
		t.Fatalf("expected exactly one non-zero band, got %d: %v", nonZero, bands)
	}
	if bands[peak] != 10 {
		t.Fatalf("bands[%d] = %v, want 10", peak, bands[peak])
	}
}

func TestNormalize_ClampsToUnitRange(t *testing.T) {
	cases := []struct{ mag, want float64 }{
		{0, 0},
		{magnitudeNormalizer, 1},
		{magnitudeNormalizer * 2, 1},
		{magnitudeNormalizer / 2, 0.5},
	}
	for _, c := range cases {
		if got := normalize(c.mag); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("normalize(%v) = %v, want %v", c.mag, got, c.want)
		}
	}
}

func TestApplyDecay_FallsExponentiallyOrRisesToNewPeak(t *testing.T) {
	prev := []float64{1.0, 0.4}
	next := []float64{0.0, 0.5}
	got := applyDecay(prev, next, 0.85)
	want := []float64{0.85, 0.5} // band0 decays from prev*0.85; band1 rises to its new peak
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
