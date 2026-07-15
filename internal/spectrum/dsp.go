package spectrum

import (
	"encoding/binary"
	"math"
	"math/cmplx"
)

const (
	windowSize          = 2048
	analysisBands       = 32
	decayFactor         = 0.85
	minFreqHz           = 20.0
	magnitudeNormalizer = 150.0
)

// hannWindow returns the n-point Hann window coefficients, used to taper
// the edges of each analysis window before FFT to reduce spectral leakage.
func hannWindow(n int) []float64 {
	w := make([]float64, n)
	for i := range w {
		w[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(n-1)))
	}
	return w
}

// downmixStereoInt16LE converts 16-bit little-endian stereo PCM bytes (as
// read from the decoder) into mono float64 samples in [-1.0, 1.0]. Any
// trailing bytes that don't form a complete 4-byte stereo frame are
// dropped.
func downmixStereoInt16LE(pcm []byte) []float64 {
	n := len(pcm) - len(pcm)%4
	out := make([]float64, n/4)
	for i := 0; i < n; i += 4 {
		l := int16(binary.LittleEndian.Uint16(pcm[i : i+2]))
		r := int16(binary.LittleEndian.Uint16(pcm[i+2 : i+4]))
		out[i/4] = (float64(l) + float64(r)) / 2 / 32768.0
	}
	return out
}

// logBandEdges returns bars+1 frequency edges (Hz), log-spaced between lo
// and hi, defining bars half-open buckets [edges[i], edges[i+1]).
func logBandEdges(bars int, lo, hi float64) []float64 {
	edges := make([]float64, bars+1)
	logLo := math.Log10(lo)
	logHi := math.Log10(hi)
	step := (logHi - logLo) / float64(bars)
	for i := range edges {
		edges[i] = math.Pow(10, logLo+step*float64(i))
	}
	return edges
}

// bandForFreq returns the index of the bucket freq falls into, or -1 if
// freq is below the lowest edge. A freq at or above the highest edge
// clamps into the last bucket.
func bandForFreq(freq float64, edges []float64) int {
	for i := 0; i < len(edges)-1; i++ {
		if freq >= edges[i] && freq < edges[i+1] {
			return i
		}
	}
	if freq >= edges[len(edges)-1] {
		return len(edges) - 2
	}
	return -1
}

// bucketMagnitudes averages FFT coefficient magnitudes (skipping the DC
// term at index 0) into bars log-spaced frequency buckets covering
// [minFreqHz, sampleRate/2]. coeffs is expected to have length n/2+1 for
// an n-point real FFT (as returned by (*fourier.FFT).Coefficients).
func bucketMagnitudes(coeffs []complex128, sampleRate int, bars int) []float64 {
	raw := make([]float64, bars)
	counts := make([]int, bars)
	nyquist := float64(sampleRate) / 2
	edges := logBandEdges(bars, minFreqHz, nyquist)
	n := 2 * (len(coeffs) - 1)

	for i := 1; i < len(coeffs); i++ {
		freq := float64(i) / float64(n) * float64(sampleRate)
		if freq > nyquist {
			break
		}
		band := bandForFreq(freq, edges)
		if band < 0 {
			continue
		}
		raw[band] += cmplx.Abs(coeffs[i])
		counts[band]++
	}
	for i := range raw {
		if counts[i] > 0 {
			raw[i] /= float64(counts[i])
		}
	}
	fillEmptyBands(raw, counts)
	return raw
}

// fillEmptyBands linearly interpolates bands that received no FFT bin
// (counts[i]==0, raw[i] left at its zero value) from their nearest
// non-empty neighbors. This happens for narrow low-frequency bands when
// the log-spaced band width is smaller than the FFT's bin spacing
// (sampleRate/windowSize) — e.g. with the analyzer's real windowSize=2048
// at 44100Hz, several of the 32 bands never get a bin. Without this fill
// those bands would read 0 forever regardless of audio content, showing
// up as permanently empty visualizer columns. A band with a non-empty
// neighbor on only one side just copies that side's value.
func fillEmptyBands(raw []float64, counts []int) {
	for i := range raw {
		if counts[i] > 0 {
			continue
		}
		prevIdx, prevVal, havePrev := -1, 0.0, false
		for j := i - 1; j >= 0; j-- {
			if counts[j] > 0 {
				prevIdx, prevVal, havePrev = j, raw[j], true
				break
			}
		}
		nextIdx, nextVal, haveNext := -1, 0.0, false
		for j := i + 1; j < len(raw); j++ {
			if counts[j] > 0 {
				nextIdx, nextVal, haveNext = j, raw[j], true
				break
			}
		}
		switch {
		case havePrev && haveNext:
			t := float64(i-prevIdx) / float64(nextIdx-prevIdx)
			raw[i] = prevVal + (nextVal-prevVal)*t
		case havePrev:
			raw[i] = prevVal
		case haveNext:
			raw[i] = nextVal
		}
	}
}

// normalize maps a raw averaged magnitude to [0.0, 1.0], clamping at the
// heuristic magnitudeNormalizer ceiling.
func normalize(mag float64) float64 {
	v := mag / magnitudeNormalizer
	if v > 1 {
		return 1
	}
	if v < 0 {
		return 0
	}
	return v
}

// applyDecay returns, per band, the larger of the new value or the
// previous value scaled by factor — a fast attack / smooth exponential
// decay envelope so bars don't flicker frame to frame.
func applyDecay(prev, next []float64, factor float64) []float64 {
	out := make([]float64, len(next))
	for i := range next {
		decayed := 0.0
		if i < len(prev) {
			decayed = prev[i] * factor
		}
		out[i] = math.Max(next[i], decayed)
	}
	return out
}
