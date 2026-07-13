# Live Equalizer Visualization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a live, multi-band equalizer visualization to the TUI, driven by the actual decoded PCM audio, toggleable and theme-consistent.

**Architecture:** A new `internal/spectrum` package taps raw PCM in `RealPlayer`'s read loop via a non-blocking fan-out, runs FFT/bucketing/decay on a dedicated goroutine, and exposes smoothed band values through a new `Player.Spectrum()` method. The UI polls that method on a fast `tea.Tick` and renders a gradient-colored bar box, gated by a persisted config toggle.

**Tech Stack:** Go 1.26, Bubble Tea, Lipgloss, go-colorful (already transitive), `gonum.org/v1/gonum/dsp/fourier` (new).

## Global Constraints

- Pure Go only — no cgo, no external binaries/processes.
- Go module version floor: `go 1.26.5` (see `go.mod`).
- Run `go mod tidy` immediately after adding any new direct import — `go get` alone leaves it marked `// indirect` until tidy runs (project CLAUDE.md gotcha, previously caught by review twice).
- Any test that calls `config.Save`/`config.Load` MUST set `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` first, or it will overwrite the real `~/.config/somafm-player/config.json`.
- No TTY/audio hardware in sandboxed agent environments — visual rendering and actual audio-reactive behavior can only be confirmed by a human running `go run .` in a real terminal; every task below relies on unit tests for logic and defers visual confirmation to that manual step.
- MP3 streams only; AAC remains a pre-existing, out-of-scope limitation.
- `internal/player/real.go`'s read loop is load-bearing (oto singleton context, teardown-join logic) — changes there must be additive and must not alter the existing teardown-join sequencing.

---

### Task 1: Add a `Hot` gradient color to `theme.Theme`

**Files:**
- Modify: `internal/theme/theme.go`
- Test: `internal/theme/theme_test.go`

**Interfaces:**
- Produces: `theme.Theme.Hot lipgloss.Color` — new field, populated for every theme in `themes`, consumed by Task 6's `gradientColor`.

- [ ] **Step 1: Write the failing test**

Add to `internal/theme/theme_test.go`:

```go
func TestThemes_AllHaveDistinctHotColor(t *testing.T) {
	for _, name := range Order {
		th := Get(name)
		if th.Hot == "" {
			t.Errorf("%s: Hot color is empty", name)
		}
		if th.Hot == th.Accent {
			t.Errorf("%s: Hot (%s) must differ from Accent (%s)", name, th.Hot, th.Accent)
		}
		if th.Hot == th.Muted {
			t.Errorf("%s: Hot (%s) must differ from Muted (%s)", name, th.Hot, th.Muted)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/theme/... -run TestThemes_AllHaveDistinctHotColor -v`
Expected: FAIL — `th.Hot` is the zero value `""`, so the "empty" check fails first.

- [ ] **Step 3: Add the `Hot` field and values**

In `internal/theme/theme.go`, add `Hot` to the struct and to every entry in `themes`:

```go
type Theme struct {
	Name       string
	Background lipgloss.Color
	Foreground lipgloss.Color
	Accent     lipgloss.Color
	Border     lipgloss.Color
	Muted      lipgloss.Color
	Hot        lipgloss.Color
}

var Order = []string{
	"Nord",
	"Dracula",
	"Gruvbox",
	"Tokyo Night",
	"Solarized Dark",
	"Solarized Light",
}

var themes = map[string]Theme{
	"Nord":            {Name: "Nord", Background: "#2E3440", Foreground: "#D8DEE9", Accent: "#88C0D0", Border: "#4C566A", Muted: "#4C566A", Hot: "#BF616A"},
	"Dracula":         {Name: "Dracula", Background: "#282A36", Foreground: "#F8F8F2", Accent: "#BD93F9", Border: "#44475A", Muted: "#6272A4", Hot: "#FF5555"},
	"Gruvbox":         {Name: "Gruvbox", Background: "#282828", Foreground: "#EBDBB2", Accent: "#FE8019", Border: "#504945", Muted: "#928374", Hot: "#FB4934"},
	"Tokyo Night":     {Name: "Tokyo Night", Background: "#1A1B26", Foreground: "#C0CAF5", Accent: "#7AA2F7", Border: "#3B4261", Muted: "#565F89", Hot: "#F7768E"},
	"Solarized Dark":  {Name: "Solarized Dark", Background: "#002B36", Foreground: "#839496", Accent: "#268BD2", Border: "#073642", Muted: "#586E75", Hot: "#DC322F"},
	"Solarized Light": {Name: "Solarized Light", Background: "#FDF6E3", Foreground: "#657B83", Accent: "#268BD2", Border: "#EEE8D5", Muted: "#93A1A1", Hot: "#DC322F"},
}
```

(Only the struct definition and the `themes` map change — `Get`, `Next`, and `Order` are untouched.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/theme/... -v`
Expected: PASS (all theme tests, including the pre-existing ones).

- [ ] **Step 5: Commit**

```bash
git add internal/theme/theme.go internal/theme/theme_test.go
git commit -m "feat(theme): add Hot gradient color for equalizer bars"
```

---

### Task 2: Add `VisualizerEnabled` to `config.Config`

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `config.Config.VisualizerEnabled bool` (JSON key `visualizerEnabled`, zero-value default `false`), consumed by Task 7 (Init/Update gating) and Task 9 (View layout).

- [ ] **Step 1: Write the failing test**

In `internal/config/config_test.go`, add a new test and extend the existing round-trip test's config literal and assertions:

```go
func TestDefaultConfig_VisualizerDisabledByDefault(t *testing.T) {
	if DefaultConfig().VisualizerEnabled {
		t.Fatal("DefaultConfig().VisualizerEnabled = true, want false (opt-in feature)")
	}
}
```

Modify `TestSaveThenLoad_RoundTrips`'s `cfg` literal to include `VisualizerEnabled: true`, and add an assertion:

```go
	cfg := Config{
		LastChannel:        "Drone Zone",
		Volume:             65,
		Muted:              false,
		Theme:              "Dracula",
		BookmarkedChannels: []string{"Drone Zone", "Groove Salad"},
		BookmarkedTunes: []BookmarkedTune{
			{Title: "Track", Artist: "Artist", Channel: "Drone Zone", BookmarkedAt: time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)},
		},
		VisualizerEnabled: true,
	}
```

```go
	if loaded.VisualizerEnabled != cfg.VisualizerEnabled {
		t.Fatalf("loaded.VisualizerEnabled = %v, want %v", loaded.VisualizerEnabled, cfg.VisualizerEnabled)
	}
```
(Add this check right after the existing `LastChannel`/`Volume`/`Theme` assertion block.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/... -v`
Expected: FAIL — `TestDefaultConfig_VisualizerDisabledByDefault` fails to compile/reference `VisualizerEnabled` (unknown field), and the round-trip test fails the same way.

- [ ] **Step 3: Add the field**

In `internal/config/config.go`:

```go
type Config struct {
	LastChannel        string           `json:"lastChannel"`
	Volume             int              `json:"volume"`
	Muted              bool             `json:"muted"`
	Theme              string           `json:"theme"`
	BookmarkedChannels []string         `json:"bookmarkedChannels"`
	BookmarkedTunes    []BookmarkedTune `json:"bookmarkedTunes"`
	VisualizerEnabled  bool             `json:"visualizerEnabled"`
}
```

`DefaultConfig()` is unchanged — Go's zero value already makes `VisualizerEnabled` default to `false`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add opt-in VisualizerEnabled setting"
```

---

### Task 3: `internal/spectrum` — pure DSP helpers (windowing, downmix, bucketing, decay)

**Files:**
- Create: `internal/spectrum/dsp.go`
- Test: `internal/spectrum/dsp_test.go`

**Interfaces:**
- Produces (package-private, consumed by Task 4 within the same package):
  - `hannWindow(n int) []float64`
  - `downmixStereoInt16LE(pcm []byte) []float64`
  - `logBandEdges(bars int, lo, hi float64) []float64`
  - `bandForFreq(freq float64, edges []float64) int`
  - `bucketMagnitudes(coeffs []complex128, sampleRate int, bars int) []float64`
  - `normalize(mag float64) float64`
  - `applyDecay(prev, next []float64, factor float64) []float64`
  - Constants: `windowSize = 2048`, `analysisBands = 32`, `decayFactor = 0.85`, `minFreqHz = 20.0`, `magnitudeNormalizer = 150.0`
- No new dependencies — this task is pure `math`/`math/cmplx`/`encoding/binary`, deliberately kept gonum-free so it's fast and trivial to unit test.

- [ ] **Step 1: Write the failing tests**

Create `internal/spectrum/dsp_test.go`:

```go
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
	binary.LittleEndian.PutUint16(pcm[2:], uint16(int16(-1000)))
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/spectrum/... -v`
Expected: FAIL to compile — package `internal/spectrum` and all referenced functions/constants don't exist yet.

- [ ] **Step 3: Implement `dsp.go`**

Create `internal/spectrum/dsp.go`:

```go
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
	return raw
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/spectrum/... -v`
Expected: PASS (all `dsp_test.go` tests)

- [ ] **Step 5: Commit**

```bash
git add internal/spectrum/dsp.go internal/spectrum/dsp_test.go
git commit -m "feat(spectrum): add pure DSP helpers for windowing, bucketing, decay"
```

---

### Task 4: `internal/spectrum` — `Analyzer` (goroutine, non-blocking fan-out, FFT integration)

**Files:**
- Create: `internal/spectrum/analyzer.go`
- Test: `internal/spectrum/analyzer_test.go`
- Modify: `go.mod`, `go.sum` (via `go mod tidy`)

**Interfaces:**
- Consumes: everything from Task 3 (`hannWindow`, `downmixStereoInt16LE`, `bucketMagnitudes`, `normalize`, `applyDecay`, `windowSize`, `analysisBands`, `decayFactor`).
- Produces (exported, consumed by Task 5):
  - `spectrum.New(sampleRate int) *Analyzer`
  - `(*Analyzer).Feed(pcm []byte)` — non-blocking; accepts 16-bit LE stereo PCM bytes.
  - `(*Analyzer).Bands() []float64` — defensive copy, length `analysisBands`, values in `[0.0, 1.0]`.
  - `(*Analyzer).Close()` — stops the background goroutine; safe to call exactly once.

- [ ] **Step 1: Add the gonum dependency**

```bash
go get gonum.org/v1/gonum/dsp/fourier
go mod tidy
```

Expected: `go.mod` gains `gonum.org/v1/gonum` as a **direct** requirement; `go.sum` is updated.

- [ ] **Step 2: Write the failing tests**

Create `internal/spectrum/analyzer_test.go`:

```go
package spectrum

import (
	"encoding/binary"
	"math"
	"testing"
	"time"
)

func sineWavePCM(freq float64, sampleRate, samples int) []byte {
	pcm := make([]byte, samples*4)
	for i := 0; i < samples; i++ {
		v := math.Sin(2 * math.Pi * freq * float64(i) / float64(sampleRate))
		s := int16(v * 20000)
		binary.LittleEndian.PutUint16(pcm[i*4:], uint16(s))
		binary.LittleEndian.PutUint16(pcm[i*4+2:], uint16(s))
	}
	return pcm
}

func waitForNonZeroBands(a *Analyzer, timeout time.Duration) []float64 {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		bands := a.Bands()
		for _, v := range bands {
			if v != 0 {
				return bands
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	return a.Bands()
}

func TestAnalyzer_BandsInitiallyAllZero(t *testing.T) {
	a := New(44100)
	defer a.Close()

	bands := a.Bands()
	if len(bands) != analysisBands {
		t.Fatalf("len(Bands()) = %d, want %d", len(bands), analysisBands)
	}
	for i, v := range bands {
		if v != 0 {
			t.Errorf("bands[%d] = %v, want 0 before any Feed", i, v)
		}
	}
}

func TestAnalyzer_BandsReturnsDefensiveCopy(t *testing.T) {
	a := New(44100)
	defer a.Close()

	got := a.Bands()
	got[0] = 99

	again := a.Bands()
	if again[0] == 99 {
		t.Fatal("mutating a returned Bands() slice affected the analyzer's internal state")
	}
}

func TestAnalyzer_FeedSineWaveProducesPeakInExpectedBand(t *testing.T) {
	a := New(44100)
	defer a.Close()

	pcm := sineWavePCM(440.0, 44100, windowSize)
	a.Feed(pcm)

	bands := waitForNonZeroBands(a, 2*time.Second)

	peak := 0
	for i, v := range bands {
		if v > bands[peak] {
			peak = i
		}
	}
	if peak != 14 {
		t.Fatalf("peak band = %d, want 14 (~440Hz bucket at 44.1kHz/2048-sample window); bands=%v", peak, bands)
	}
}

func TestAnalyzer_CloseThenFeedDoesNotPanic(t *testing.T) {
	a := New(44100)
	a.Close()
	a.Feed(make([]byte, 16))
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/spectrum/... -run TestAnalyzer -v`
Expected: FAIL to compile — `Analyzer`, `New`, `Feed`, `Bands`, `Close` don't exist yet.

- [ ] **Step 4: Implement `analyzer.go`**

Create `internal/spectrum/analyzer.go`:

```go
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
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/spectrum/... -v`
Expected: PASS (all tests in the package, Task 3 and Task 4 together)

- [ ] **Step 6: Commit**

```bash
git add internal/spectrum/analyzer.go internal/spectrum/analyzer_test.go go.mod go.sum
git commit -m "feat(spectrum): add Analyzer with non-blocking PCM fan-out and FFT pipeline"
```

---

### Task 5: Wire `Spectrum()` into the `Player` interface, `FakePlayer`, and `RealPlayer`

**Files:**
- Modify: `internal/player/player.go`
- Modify: `internal/player/fake.go`
- Test: `internal/player/fake_test.go`
- Modify: `internal/player/real.go`
- Test: `internal/player/real_test.go` (new)

**Interfaces:**
- Consumes: `spectrum.New(sampleRate int) *spectrum.Analyzer`, `(*spectrum.Analyzer).Feed([]byte)`, `.Bands() []float64`, `.Close()` (Task 4).
- Produces: `Player.Spectrum() []float64` — consumed by Task 7's `handleVisualizerTick`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/player/fake_test.go`:

```go
func TestFakePlayer_SpectrumReturnsNil(t *testing.T) {
	p := NewFakePlayer()
	if got := p.Spectrum(); got != nil {
		t.Fatalf("Spectrum() = %v, want nil", got)
	}
}
```

Create `internal/player/real_test.go`:

```go
package player

import "testing"

func TestRealPlayer_SpectrumReturnsNilWhenNoStreamActive(t *testing.T) {
	p := NewRealPlayer()
	if got := p.Spectrum(); got != nil {
		t.Fatalf("Spectrum() = %v, want nil when no stream is playing", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/player/... -v`
Expected: FAIL to compile — `Spectrum` is not a method on `*FakePlayer`/`*RealPlayer`, and the `Player` interface doesn't declare it yet.

- [ ] **Step 3: Add `Spectrum()` to the interface**

In `internal/player/player.go`:

```go
type Player interface {
	Play(streamURL string)
	Stop()
	SetVolume(percent int)
	SetMuted(muted bool)
	Messages() <-chan Msg
	Spectrum() []float64
}
```

- [ ] **Step 4: Implement `FakePlayer.Spectrum()`**

Add to `internal/player/fake.go`:

```go
// Spectrum always returns nil: FakePlayer does no real decoding, so there
// is no audio to analyze. UI code consuming this exercises the nil
// ("nothing playing" / flat bars) path deterministically in tests.
func (p *FakePlayer) Spectrum() []float64 {
	return nil
}
```

- [ ] **Step 5: Implement `RealPlayer.Spectrum()` and wire the analyzer into `playOnce`**

In `internal/player/real.go`, add the import and struct field:

```go
import (
	"context"
	"strings"
	"sync"
	"time"

	mp3 "github.com/hajimehoshi/go-mp3"
	"github.com/hajimehoshi/oto"
	"github.com/romantomjak/shoutcast"

	"github.com/jonasbn/somafm-player/internal/spectrum"
)
```

```go
type RealPlayer struct {
	msgs   chan Msg
	mu     sync.Mutex
	volume int
	muted  bool

	cancel   context.CancelFunc
	stream   *shoutcast.Stream
	done     chan struct{}
	analyzer *spectrum.Analyzer
}
```

Add the accessor method (place it near `SetMuted`):

```go
// Spectrum returns the current smoothed frequency-band values from the
// active stream's analyzer, or nil if nothing is currently playing.
func (p *RealPlayer) Spectrum() []float64 {
	p.mu.Lock()
	a := p.analyzer
	p.mu.Unlock()
	if a == nil {
		return nil
	}
	return a.Bands()
}
```

In `playOnce`, create the analyzer right after the oto player is set up, and feed it in the read loop:

```go
	otoPlayer := otoCtx.NewPlayer()
	defer otoPlayer.Close()

	analyzer := spectrum.New(decoder.SampleRate())
	p.mu.Lock()
	p.analyzer = analyzer
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		if p.analyzer == analyzer {
			p.analyzer = nil
		}
		p.mu.Unlock()
		analyzer.Close()
	}()

	vr := newVolumeReader(decoder, p.volumeFactor)
	p.msgs <- ReconnectedMsg{}

	buf := make([]byte, 4096)
	for {
		if ctx.Err() != nil {
			return nil
		}
		n, readErr := vr.Read(buf)
		if n > 0 {
			if _, writeErr := otoPlayer.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			analyzer.Feed(buf[:n])
		}
		if readErr != nil {
			return readErr
		}
	}
}
```

This adds exactly one non-blocking call (`analyzer.Feed`) to the existing read loop, after the existing `otoPlayer.Write` — it does not alter the existing teardown-join sequencing in `Play()`, and the analyzer's own lifetime is scoped entirely to this `playOnce` call via the `defer`.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/player/... ./internal/spectrum/... ./... -v`
Expected: PASS across the whole module (confirms the interface change didn't break any other `Player` consumer).

- [ ] **Step 7: Commit**

```bash
git add internal/player/player.go internal/player/fake.go internal/player/fake_test.go internal/player/real.go internal/player/real_test.go
git commit -m "feat(player): expose live spectrum data via Player.Spectrum()"
```

---

### Task 6: `internal/ui` — visualizer rendering primitives

**Files:**
- Create: `internal/ui/visualizer.go`
- Test: `internal/ui/visualizer_test.go`
- Modify: `go.mod`, `go.sum` (via `go mod tidy`, promoting `go-colorful` to direct)

**Interfaces:**
- Consumes: `theme.Theme.Muted/Accent/Hot` (Task 1).
- Produces (consumed by Task 9): `displayBarCount(width int) int`, `resampleBands(bands []float64, barCount int) []float64`, `barChar(v float64) rune`, `gradientColor(v float64, t theme.Theme) lipgloss.Color`, constants `minDisplayBars = 8`, `maxDisplayBars = 32`.
- Deliberately does **not** yet reference `Model.bands` (that field doesn't exist until Task 7) — these are pure, Model-independent functions so this task compiles and is testable in isolation.

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/visualizer_test.go`:

```go
package ui

import (
	"math"
	"testing"

	"github.com/lucasb-eyer/go-colorful"

	"github.com/jonasbn/somafm-player/internal/theme"
)

func TestBarChar_MapsLevelsAcrossRange(t *testing.T) {
	if got := barChar(0); got != barLevels[0] {
		t.Errorf("barChar(0) = %q, want %q", got, barLevels[0])
	}
	if got := barChar(1); got != barLevels[len(barLevels)-1] {
		t.Errorf("barChar(1) = %q, want %q", got, barLevels[len(barLevels)-1])
	}
}

func TestResampleBands_AveragesContiguousGroups(t *testing.T) {
	bands := []float64{0, 0, 1, 1} // 4 input bands -> 2 output bars
	got := resampleBands(bands, 2)
	want := []float64{0, 1}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestResampleBands_NilBandsProducesZeroFilledSlice(t *testing.T) {
	got := resampleBands(nil, 8)
	if len(got) != 8 {
		t.Fatalf("len = %d, want 8", len(got))
	}
	for i, v := range got {
		if v != 0 {
			t.Errorf("got[%d] = %v, want 0", i, v)
		}
	}
}

func TestDisplayBarCount_ClampsToBounds(t *testing.T) {
	if got := displayBarCount(3); got != minDisplayBars {
		t.Errorf("displayBarCount(3) = %d, want %d", got, minDisplayBars)
	}
	if got := displayBarCount(100); got != maxDisplayBars {
		t.Errorf("displayBarCount(100) = %d, want %d", got, maxDisplayBars)
	}
	if got := displayBarCount(20); got != 20 {
		t.Errorf("displayBarCount(20) = %d, want 20", got)
	}
}

func TestGradientColor_EndpointsMatchThemeStops(t *testing.T) {
	th := theme.Get("Nord")
	cases := []struct {
		v    float64
		want string
	}{
		{0, string(th.Muted)},
		{0.5, string(th.Accent)},
		{1, string(th.Hot)},
	}
	for _, c := range cases {
		got := gradientColor(c.v, th)
		gotC, _ := colorful.Hex(string(got))
		wantC, _ := colorful.Hex(c.want)
		if dist := gotC.DistanceLab(wantC); dist > 0.01 {
			t.Errorf("gradientColor(%v) = %v, want close to %v (Lab distance %v)", c.v, got, c.want, dist)
		}
	}
}

func TestGradientColor_ClampsOutOfRangeInput(t *testing.T) {
	th := theme.Get("Nord")
	if got, zero := gradientColor(-1, th), gradientColor(0, th); got != zero {
		t.Errorf("gradientColor(-1) = %v, want same as gradientColor(0) = %v", got, zero)
	}
	if got, one := gradientColor(2, th), gradientColor(1, th); got != one {
		t.Errorf("gradientColor(2) = %v, want same as gradientColor(1) = %v", got, one)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/... -run 'TestBarChar|TestResampleBands|TestDisplayBarCount|TestGradientColor' -v`
Expected: FAIL to compile — `barChar`, `barLevels`, `resampleBands`, `displayBarCount`, `gradientColor`, `minDisplayBars`, `maxDisplayBars` don't exist yet.

- [ ] **Step 3: Implement `visualizer.go`**

Create `internal/ui/visualizer.go`:

```go
package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/jonasbn/somafm-player/internal/theme"
)

// minDisplayBars/maxDisplayBars bound how many bars the visualizer box
// renders, derived from its available width so it degrades gracefully on
// narrow terminals without getting absurdly dense on ultrawide ones.
const (
	minDisplayBars = 8
	maxDisplayBars = 32
)

var barLevels = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// displayBarCount clamps width to [minDisplayBars, maxDisplayBars].
func displayBarCount(width int) int {
	n := width
	if n < minDisplayBars {
		n = minDisplayBars
	}
	if n > maxDisplayBars {
		n = maxDisplayBars
	}
	return n
}

// resampleBands maps an arbitrary-length bands slice onto barCount output
// values by averaging contiguous groups. A nil or empty bands (nothing
// playing) produces a zero-filled slice of length barCount, rendering as
// flat bars rather than an empty/collapsed box.
func resampleBands(bands []float64, barCount int) []float64 {
	out := make([]float64, barCount)
	if len(bands) == 0 || barCount <= 0 {
		return out
	}
	for i := range out {
		lo := i * len(bands) / barCount
		hi := (i + 1) * len(bands) / barCount
		if hi <= lo {
			hi = lo + 1
		}
		if hi > len(bands) {
			hi = len(bands)
		}
		sum := 0.0
		for j := lo; j < hi; j++ {
			sum += bands[j]
		}
		out[i] = sum / float64(hi-lo)
	}
	return out
}

// barChar maps a 0.0-1.0 fill level to one of 8 sub-cell block characters.
func barChar(v float64) rune {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	idx := int(v*float64(len(barLevels)-1) + 0.5)
	return barLevels[idx]
}

// gradientColor interpolates a bar's color across three stops — Muted (0),
// Accent (0.5), Hot (1) — using perceptual Lab blending so the transition
// reads as smooth rather than muddy.
func gradientColor(v float64, t theme.Theme) lipgloss.Color {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	lo, _ := colorful.Hex(string(t.Muted))
	mid, _ := colorful.Hex(string(t.Accent))
	hi, _ := colorful.Hex(string(t.Hot))

	var c colorful.Color
	if v <= 0.5 {
		c = lo.BlendLab(mid, v/0.5)
	} else {
		c = mid.BlendLab(hi, (v-0.5)/0.5)
	}
	return lipgloss.Color(c.Hex())
}
```

- [ ] **Step 4: Promote go-colorful to a direct dependency**

```bash
go mod tidy
```

Expected: `github.com/lucasb-eyer/go-colorful` moves from `// indirect` to a direct `require` line in `go.mod`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/ui/... -v`
Expected: PASS (new visualizer tests; all pre-existing `internal/ui` tests remain green since nothing else changed yet).

- [ ] **Step 6: Commit**

```bash
git add internal/ui/visualizer.go internal/ui/visualizer_test.go go.mod go.sum
git commit -m "feat(ui): add equalizer bar rendering primitives (gradient, resampling)"
```

---

### Task 7: `internal/ui` — `Model.bands` field and the visualizer tick loop

**Files:**
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/timers.go`
- Test: `internal/ui/timers_test.go`
- Test: `internal/ui/model_test.go`

**Interfaces:**
- Consumes: `player.Player.Spectrum() []float64` (Task 5), `config.Config.VisualizerEnabled` (Task 2).
- Produces: `Model.bands []float64` field — consumed by Task 9's `renderVisualizerBox`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/ui/timers_test.go`:

```go
func TestHandleVisualizerTick_PullsBandsFromPlayer(t *testing.T) {
	m := newTestModel() // FakePlayer.Spectrum() always returns nil
	m.bands = []float64{0.9, 0.9}

	m = m.handleVisualizerTick()

	if m.bands != nil {
		t.Fatalf("bands = %v, want nil (pulled from FakePlayer.Spectrum())", m.bands)
	}
}
```

Add to `internal/ui/model_test.go`:

```go
func TestUpdate_VisualizerTickMsgReschedulesWhenEnabled(t *testing.T) {
	m := newTestModel()
	m.cfg.VisualizerEnabled = true

	_, cmd := m.Update(visualizerTickMsg(time.Now()))
	if cmd == nil {
		t.Fatal("expected a reschedule cmd when visualizer is enabled")
	}
}

func TestUpdate_VisualizerTickMsgNoOpAndNoRescheduleWhenDisabled(t *testing.T) {
	m := newTestModel()
	m.cfg.VisualizerEnabled = false
	m.bands = []float64{0.5}

	next, cmd := m.Update(visualizerTickMsg(time.Now()))
	m = next.(Model)

	if len(m.bands) != 1 {
		t.Fatalf("bands = %v, want unchanged when disabled", m.bands)
	}
	if cmd != nil {
		t.Fatal("expected no reschedule cmd when visualizer is disabled")
	}
}

func TestInit_SchedulesVisualizerTickWhenEnabledInConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.VisualizerEnabled = true
	m := New(cfg, nil, player.NewFakePlayer(), history.New(5))

	batch, ok := m.Init()().(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init()() = %T, want tea.BatchMsg", m.Init()())
	}
	if len(batch) != 3 {
		t.Fatalf("len(batch) = %d, want 3 (waitForPlayerMsg, tickCmd, visualizerTickCmd)", len(batch))
	}
}

func TestInit_DoesNotScheduleVisualizerTickWhenDisabled(t *testing.T) {
	m := newTestModel() // VisualizerEnabled=false by default

	batch, ok := m.Init()().(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init()() = %T, want tea.BatchMsg", m.Init()())
	}
	if len(batch) != 2 {
		t.Fatalf("len(batch) = %d, want 2 (waitForPlayerMsg, tickCmd)", len(batch))
	}
}
```

Add `"time"` to `model_test.go`'s imports if not already present (it is not — `model_test.go` currently imports `testing`, `tea`, `channels`, `config`, `history`, `player`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/... -v`
Expected: FAIL to compile — `handleVisualizerTick`, `visualizerTickMsg`, `visualizerTickCmd`, and `Model.bands` don't exist yet.

- [ ] **Step 3: Add `bands` to `Model` and wire `Init`**

In `internal/ui/model.go`, add the field to the `Model` struct (near `nowPlaying`):

```go
type Model struct {
	cfg      config.Config
	channels []channels.Channel

	channelSelected int
	tuneSelected    int
	channelsFilter  channelsFilter
	tunesMode       tunesMode
	focus           focusArea
	width           int

	player player.Player
	hist   *history.History

	nowPlaying nowPlayingState
	bands      []float64
	errMsg     string
	quitting   bool

	sessionStarted time.Time
	session        string
}
```

Update `Init`:

```go
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{waitForPlayerMsg(m.player), tickCmd()}
	if m.cfg.VisualizerEnabled {
		cmds = append(cmds, visualizerTickCmd())
	}
	if m.cfg.LastChannel != "" {
		for _, ch := range m.channels {
			if ch.Title == m.cfg.LastChannel {
				cmds = append(cmds, resolveAndPlayCmd(ch))
				break
			}
		}
	}
	return tea.Batch(cmds...)
}
```

Add the dispatch in `Update` (immediately after the existing `if t, ok := msg.(tickMsg); ok { ... }` block):

```go
	if _, ok := msg.(visualizerTickMsg); ok {
		if !m.cfg.VisualizerEnabled {
			return m, nil
		}
		return m.handleVisualizerTick(), visualizerTickCmd()
	}
```

- [ ] **Step 4: Add the tick command and handler to `timers.go`**

In `internal/ui/timers.go`, add:

```go
const visualizerTickInterval = 50 * time.Millisecond

type visualizerTickMsg time.Time

func visualizerTickCmd() tea.Cmd {
	return tea.Tick(visualizerTickInterval, func(t time.Time) tea.Msg {
		return visualizerTickMsg(t)
	})
}

func (m Model) handleVisualizerTick() Model {
	m.bands = m.player.Spectrum()
	return m
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/ui/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/ui/model.go internal/ui/timers.go internal/ui/timers_test.go internal/ui/model_test.go
git commit -m "feat(ui): wire visualizer tick loop and pull live bands into Model"
```

---

### Task 8: `internal/ui` — `v` keybinding to toggle the visualizer

**Files:**
- Create: `internal/ui/visualizer_actions.go`
- Test: `internal/ui/visualizer_actions_test.go`
- Modify: `internal/ui/model.go`

**Interfaces:**
- Consumes: `Model.cfg.VisualizerEnabled` (Task 2), `visualizerTickCmd()` (Task 7).
- Produces: `(m Model) toggleVisualizer() (Model, tea.Cmd)`, wired to the `"v"` key in `Update`.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/visualizer_actions_test.go`:

```go
package ui

import "testing"

func TestUpdate_VKeyTogglesVisualizerAndSchedulesTickWhenEnabling(t *testing.T) {
	m := newTestModel()
	m.cfg.VisualizerEnabled = false

	next, cmd := m.Update(key("v"))
	m = next.(Model)

	if !m.cfg.VisualizerEnabled {
		t.Fatal("VisualizerEnabled = false after v, want true")
	}
	if cmd == nil {
		t.Fatal("expected a tick-scheduling cmd when enabling the visualizer")
	}
}

func TestUpdate_VKeyDisablingReturnsNoCmd(t *testing.T) {
	m := newTestModel()
	m.cfg.VisualizerEnabled = true

	next, cmd := m.Update(key("v"))
	m = next.(Model)

	if m.cfg.VisualizerEnabled {
		t.Fatal("VisualizerEnabled = true after v, want false")
	}
	if cmd != nil {
		t.Fatal("expected no cmd when disabling the visualizer")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/... -run TestUpdate_VKey -v`
Expected: FAIL — `"v"` is not a handled key in `Update` yet, so `VisualizerEnabled` never flips.

- [ ] **Step 3: Implement `toggleVisualizer` and wire the key**

Create `internal/ui/visualizer_actions.go`:

```go
package ui

import tea "github.com/charmbracelet/bubbletea"

// toggleVisualizer flips the visualizer on/off. Turning it on kicks off
// the fast visualizer tick loop (Init only schedules that loop once, at
// startup, based on the config value at that time); turning it off simply
// stops rescheduling — handled by the visualizerTickMsg case in Update.
func (m Model) toggleVisualizer() (Model, tea.Cmd) {
	m.cfg.VisualizerEnabled = !m.cfg.VisualizerEnabled
	if m.cfg.VisualizerEnabled {
		return m, visualizerTickCmd()
	}
	return m, nil
}
```

In `internal/ui/model.go`'s `Update`, add a case alongside the other single-letter keys (e.g. next to `case "t":`):

```go
		case "v":
			return m.toggleVisualizer()
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ui/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/visualizer_actions.go internal/ui/visualizer_actions_test.go internal/ui/model.go
git commit -m "feat(ui): add v keybinding to toggle the visualizer"
```

---

### Task 9: `internal/ui` — render the visualizer box in the layout

**Files:**
- Modify: `internal/ui/visualizer.go`
- Modify: `internal/ui/view.go`
- Test: `internal/ui/view_test.go`

**Interfaces:**
- Consumes: `displayBarCount`, `resampleBands`, `barChar`, `gradientColor` (Task 6); `Model.bands` (Task 7); `Model.cfg.VisualizerEnabled` (Task 2); `borderStyle` (existing, `view.go`).
- Produces: `(m Model) renderVisualizerBox(t theme.Theme, width int) string`, `(m Model) fullBoxWidth() int`; `View()` renders the box between Now Playing and the Channels/Tunes row when enabled.

- [ ] **Step 1: Write the failing tests**

Add to `internal/ui/view_test.go`:

```go
func TestFullBoxWidth_SubtractsSingleBoxDecorationOnly(t *testing.T) {
	m := newTestModel()
	m.width = 100

	got := m.fullBoxWidth()

	// 100 total - 4 (border+padding decoration for one box) = 96
	if got != 96 {
		t.Fatalf("fullBoxWidth() = %d, want 96 for a 100-column terminal", got)
	}
}

func TestView_HidesVisualizerBoxByDefault(t *testing.T) {
	m := newTestModel() // config.DefaultConfig() has VisualizerEnabled=false
	out := m.View()
	if strings.ContainsAny(out, "▁▂▃▄▅▆▇█") {
		t.Fatalf("View() output contains visualizer bar characters while disabled:\n%s", out)
	}
}

func TestView_ShowsVisualizerBoxWhenEnabled(t *testing.T) {
	m := newTestModel()
	m.cfg.VisualizerEnabled = true
	out := m.View()
	if !strings.ContainsAny(out, "▁▂▃▄▅▆▇█") {
		t.Fatalf("View() output missing visualizer bar characters while enabled:\n%s", out)
	}
}

func TestView_FooterMentionsVisualizerKey(t *testing.T) {
	m := newTestModel()
	if got := m.View(); !strings.Contains(got, "v visualizer") {
		t.Fatalf("View() footer missing 'v visualizer' hint:\n%s", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/... -run 'TestFullBoxWidth|TestView_' -v`
Expected: FAIL — `fullBoxWidth` doesn't exist, `View()` never renders bar characters, and the footer doesn't mention `v`.

- [ ] **Step 3: Add `renderVisualizerBox` to `visualizer.go`**

Append to `internal/ui/visualizer.go`:

```go
func (m Model) renderVisualizerBox(t theme.Theme, width int) string {
	bars := resampleBands(m.bands, displayBarCount(width))
	var sb strings.Builder
	for _, v := range bars {
		style := lipgloss.NewStyle().Foreground(gradientColor(v, t))
		sb.WriteString(style.Render(string(barChar(v))))
	}
	return borderStyle(t, false).Width(width).Render(sb.String())
}
```

Add `"strings"` to the existing `internal/ui/visualizer.go` import block:

```go
import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/jonasbn/somafm-player/internal/theme"
)
```

- [ ] **Step 4: Update `view.go`**

In `internal/ui/view.go`, add `fullBoxWidth` near `boxWidth`:

```go
// fullBoxWidth returns the available width for a single full-width box
// (unlike boxWidth, which splits the width between two side-by-side
// boxes). Falls back to defaultWidth before the first tea.WindowSizeMsg.
func (m Model) fullBoxWidth() int {
	w := m.width
	if w <= 0 {
		w = defaultWidth
	}
	usable := w - decorationPerBox
	if usable < 2 {
		usable = 2
	}
	return usable
}
```

Replace `View()`:

```go
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	t := theme.Get(m.cfg.Theme)

	width := m.boxWidth()
	lists := lipgloss.JoinHorizontal(lipgloss.Top, m.renderChannelsBox(t, width), m.renderTunesBox(t, width))

	sections := []string{m.renderNowPlaying(t)}
	if m.cfg.VisualizerEnabled {
		sections = append(sections, m.renderVisualizerBox(t, m.fullBoxWidth()))
	}
	sections = append(sections, lists)

	footer := fmt.Sprintf("[Theme: %s]  tab focus · j/k move · enter play · b bookmark · a all/bookmarked · H/s tunes · +/- vol · m mute · t theme · v visualizer · r retry channels · q quit", t.Name)
	if m.errMsg != "" {
		footer = "Error: " + m.errMsg + "\n" + footer
	}
	sections = append(sections, footer)

	return strings.Join(sections, "\n")
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS — full module test suite green.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/visualizer.go internal/ui/view.go internal/ui/view_test.go
git commit -m "feat(ui): render the equalizer box in the layout when enabled"
```

---

## Manual verification (required — not automatable in this environment)

Per CLAUDE.md, no TTY/audio hardware is available in the sandboxed agent environment. After all 9 tasks are merged, a human must run `go run .` in a real terminal and confirm:

1. Pressing `v` shows/hides a full-width bar box between Now Playing and the Channels/Tunes row.
2. While a stream plays, bars visibly react to the music (attack/decay reads as "musical," not jittery or static).
3. Bars are colored on a gradient from the theme's Muted color (quiet) through Accent to Hot (loud), and this gradient looks reasonable across all 6 themes (cycle with `t`).
4. Resizing the terminal changes the bar count smoothly (more bars on a wider terminal, fewer on a narrower one, clamped 8–32).
5. Toggling `v` off and quitting, then relaunching, confirms the on/off state persisted (config.json's `visualizerEnabled`).
6. No audible glitches, dropouts, or added latency in playback with the visualizer on vs. off.
