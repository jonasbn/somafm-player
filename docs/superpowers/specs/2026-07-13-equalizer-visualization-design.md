# Design: Live equalizer visualization

Date: 2026-07-13
Source: `equalizer-visualization-feature.md` (repo root research notes)

## Problem

The TUI has no visual feedback tied to the actual audio being played — the
Now Playing box shows metadata (title/artist/channel/elapsed) but nothing
reflects the sound itself. Terminal radio players like `cava`/`ncmpcpp`
commonly show a live frequency-band spectrum; this feature adds one, driven
by the real decoded PCM rather than a fake/simulated animation.

## Goals

1. Render a live multi-band equalizer (bar count derived from terminal
   width) while a stream plays, reacting to the actual audio.
2. Musical smoothing (fast attack, smooth decay) — not raw per-frame jitter.
3. Zero added latency or audio glitches: visualization is a side channel to
   the existing `RealPlayer` → `oto` playback path, never blocking it.
4. Pure-Go, no cgo, consistent with the rest of the stack. MP3 streams only;
   AAC remains the pre-existing known limitation.
5. Opt-in and theme-consistent, fitting the project's existing
   config-persistence and theming conventions.

## Non-goals

- No AAC support (carried-over known limitation).
- No user-configurable DSP tuning (decay factor, window size, tick
  interval, band Hz cutoffs) in this version — hardcoded constants, YAGNI.
- No changes to the audio decode/playback path itself beyond one
  non-blocking fan-out send.

## Architecture: data flow

```
RealPlayer.playOnce() read loop (internal/player/real.go)
   │  after each otoPlayer.Write(buf[:n]):
   │  non-blocking send of buf[:n] into analyzer's PCM channel
   │  (drop-if-full policy — never blocks the audio hot path)
   ▼
spectrum.Analyzer (own goroutine, one per active stream)
   │  accumulates bytes → int16 stereo samples → mono downmix
   │  fills a 2048-sample window → Hann window → gonum real FFT
   │  → magnitude spectrum (sqrt(re²+im²))
   │  → logarithmic band bucketing → per-band exponential decay
   │  → stores result in a mutex-guarded []float64 (0.0-1.0 per band)
   ▼
Analyzer.Bands() []float64   (defensive copy, same pattern as history.Entries())
```

**New package `internal/spectrum`**, parallel to `internal/player`,
`internal/history`, etc. Owns all DSP logic (windowing, FFT, bucketing,
decay) with no dependency on Bubble Tea, oto, or shoutcast — fully
unit-testable with synthetic PCM input (e.g. a generated sine wave at a
known frequency should concentrate energy in a predictable bucket), which
sidesteps the project's "no TTY/audio hardware in sandboxed environments"
constraint since this is pure signal-processing math.

`RealPlayer` owns one `*spectrum.Analyzer` per `playOnce` call, created
alongside the decoder/oto setup and scoped to that stream's lifetime — it
doesn't need to survive across reconnects, so it adds no new complexity to
the existing teardown-join logic in `Play()`.

### Player interface change

```go
type Player interface {
    Play(streamURL string)
    Stop()
    SetVolume(percent int)
    SetMuted(muted bool)
    Messages() <-chan Msg
    Spectrum() []float64  // NEW — current band values, 0.0-1.0, nil if unavailable
}
```

- `RealPlayer.Spectrum()` delegates to its active analyzer; returns nil if
  no stream is active.
- `FakePlayer.Spectrum()` returns `nil` unconditionally — a one-line no-op,
  no existing test changes required.

## Config & persistence

`internal/config/config.go` gains one field:

```go
type Config struct {
    ...
    VisualizerEnabled bool `json:"visualizerEnabled"`
}
```

Defaults to `false` (Go zero value; `DefaultConfig()` does not set it) —
opt-in, since terminal Unicode block-character and 24-bit color rendering
varies across environments. Persisted via the existing `config.Save` call
on quit, same as `Theme`/`Volume`/`Muted`.

## Keybinding

`v` toggles `m.cfg.VisualizerEnabled`, implemented as `toggleVisualizer()`
in a new `internal/ui/visualizer_actions.go`, mirroring the existing
`cycleTheme()` pattern in `theme_actions.go`. Added to the footer help
string alongside the other single-letter bindings.

## Layout & rendering

Screen structure becomes (top to bottom):

```
renderNowPlaying                (full width, unchanged)
[ visualizer box ]              (NEW — full width, only when cfg.VisualizerEnabled)
Channels box | Tunes box        (side by side, unchanged)
footer
```

The visualizer box is a new full-width box (not split like the two list
boxes), rendered by a new `internal/ui/visualizer.go`. Its width reuses the
existing full-width calculation pattern from `view.go`; bar count is
derived from that width, clamped to 8–32 bars so it degrades gracefully on
very narrow terminals and doesn't get absurdly dense on ultrawide ones.

**Bar rendering**: each bar is one terminal column, height mapped to one of
8 sub-cell levels via `▁▂▃▄▅▆▇█`. Color is chosen per bar by its fill
fraction, interpolated across three stops — `Muted → Accent → Hot` — using
`lipgloss`/`go-colorful` (already present transitively via lipgloss).

**Silence/idle behavior**: when `Spectrum()` returns nil (nothing playing,
reconnecting, or `FakePlayer` in tests), bars render flat/empty rather than
the box disappearing — avoids layout jumping every time playback
starts/stops/reconnects.

### Theme change

`internal/theme/theme.go`'s `Theme` struct gains a `Hot` field, populated
for all 6 existing themes with a warm/bright color distinct from that
theme's `Accent`:

| Theme            | Hot       |
|------------------|-----------|
| Nord              | `#BF616A` |
| Dracula           | `#FF5555` |
| Gruvbox           | `#FB4934` |
| Tokyo Night       | `#F7768E` |
| Solarized Dark    | `#DC322F` |
| Solarized Light   | `#DC322F` |

### Fast tick

A new `visualizerTickCmd` (~50ms / ~20fps), separate from the existing 1s
`tickCmd` in `timers.go`. It is only scheduled while
`cfg.VisualizerEnabled` is true (checked when re-issuing the command from
`Update`), so there is zero polling overhead when the feature is off. Each
tick pulls `m.player.Spectrum()` into `Model` state for `View` to render —
the same "tea.Tick pulls from a shared buffer" pattern the existing elapsed-
time ticker already uses.

## DSP constants (unexported, in `internal/spectrum`)

- FFT window: 2048 samples
- Window function: Hann
- Decay factor: 0.85 per tick (`height = max(newHeight, height * 0.85)`)
- Update interval: 50ms
- Band bucketing: logarithmic, modeled after a standard 10-band graphic-EQ
  frequency table, collapsed/expanded at render time to match the runtime
  bar count (8–32)

## New dependency

`gonum.org/v1/gonum/dsp/fourier` — added as a **direct** import in
`internal/spectrum`. Per the project's CLAUDE.md gotcha, `go mod tidy` must
be run immediately after adding the import so it doesn't linger marked
`// indirect` in `go.mod`.

## Testing

- `internal/spectrum`: unit tests for windowing, FFT magnitude computation,
  log bucketing (synthetic sine-wave input at known frequencies →
  predictable bucket), and decay smoothing (monotonic decay when input
  drops to silence) — all pure math, no audio hardware needed.
- `internal/player`: `FakePlayer.Spectrum()` returns nil — covered by
  existing fake-player test patterns, no new test infrastructure.
- `internal/ui`: unit tests for `toggleVisualizer()` and for
  `visualizerTickCmd` only being scheduled when `cfg.VisualizerEnabled` is
  true, following existing `internal/ui` test conventions.
- Visual rendering (actual bar appearance, gradient correctness, terminal
  compatibility) is not unit-testable — per CLAUDE.md, requires manual
  verification via `go run .` in a real terminal with an actual stream
  playing.
