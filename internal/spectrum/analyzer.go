package spectrum

import (
	"sync"

	"gonum.org/v1/gonum/dsp/fourier"
)

// pcmChanCapacity bounds the fan-out channel between the audio read loop
// and this analyzer's goroutine. It is intentionally small: Feed drops
// chunks rather than blocking the caller once the channel is full, so a
// slow analyzer never stalls audio playback.
const pcmChanCapacity = 4

// Analyzer consumes raw PCM audio fed via Feed and exposes a smoothed,
// log-bucketed magnitude spectrum via Bands. All FFT/bucketing/decay work
// happens on a dedicated goroutine started by New; Feed and Bands are safe
// to call concurrently from other goroutines (Feed from the audio read
// loop, Bands from the UI).
type Analyzer struct {
	sampleRate int

	pcmCh chan []byte
	done  chan struct{}

	fft    *fourier.FFT
	window []float64
	accum  []float64
	filled int

	mu    sync.Mutex
	bands []float64
}

// New creates an Analyzer for a stream at the given sample rate and starts
// its background processing goroutine. Callers must call Close when the
// stream ends to stop that goroutine.
func New(sampleRate int) *Analyzer {
	a := &Analyzer{
		sampleRate: sampleRate,
		pcmCh:      make(chan []byte, pcmChanCapacity),
		done:       make(chan struct{}),
		fft:        fourier.NewFFT(windowSize),
		window:     hannWindow(windowSize),
		accum:      make([]float64, windowSize),
		bands:      make([]float64, analysisBands),
	}
	go a.run()
	return a
}

func (a *Analyzer) run() {
	for {
		select {
		case pcm := <-a.pcmCh:
			a.consume(pcm)
		case <-a.done:
			return
		}
	}
}

// Feed submits raw 16-bit-LE stereo PCM bytes for analysis. It never
// blocks: if the analyzer's goroutine is still processing a previous
// chunk, this chunk is dropped rather than stalling the caller.
func (a *Analyzer) Feed(pcm []byte) {
	cp := make([]byte, len(pcm))
	copy(cp, pcm)
	select {
	case a.pcmCh <- cp:
	default:
	}
}

// Close stops the analyzer's background goroutine. Call exactly once.
func (a *Analyzer) Close() {
	close(a.done)
}

func (a *Analyzer) consume(pcm []byte) {
	samples := downmixStereoInt16LE(pcm)
	for _, s := range samples {
		a.accum[a.filled] = s
		a.filled++
		if a.filled == windowSize {
			a.processWindow()
			a.filled = 0
		}
	}
}

func (a *Analyzer) processWindow() {
	windowed := make([]float64, windowSize)
	for i, v := range a.accum {
		windowed[i] = v * a.window[i]
	}
	coeffs := a.fft.Coefficients(nil, windowed)
	raw := bucketMagnitudes(coeffs, a.sampleRate, analysisBands)

	next := make([]float64, analysisBands)
	for i, v := range raw {
		next[i] = normalize(v)
	}

	a.mu.Lock()
	a.bands = applyDecay(a.bands, next, decayFactor)
	a.mu.Unlock()
}

// Bands returns a defensive copy of the current smoothed band values
// (length analysisBands, each in [0.0, 1.0]).
func (a *Analyzer) Bands() []float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]float64, len(a.bands))
	copy(out, a.bands)
	return out
}
