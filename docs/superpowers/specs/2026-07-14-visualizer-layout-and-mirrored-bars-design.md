# Design: Visualizer layout move + mirrored waveform bars

Date: 2026-07-14
Supersedes the layout/rendering portions of
`2026-07-13-equalizer-visualization-design.md` (the "Layout & rendering"
section there described the original full-width, single-row visualizer).

## Problem

The visualizer box, added the previous day, renders as a single row of
Unicode block characters spanning the full terminal width, on its own row
below Now Playing. At 1 content row (3 rows including border) it reads as
"very small" relative to the rest of the UI, and its placement — a full row
of its own — doesn't visually associate it with the player it's reacting to.

## Goals

1. Move the visualizer box to sit beside the Now Playing box (same row),
   so the two panels read as one "now playing + visualization" unit.
2. Grow the visualizer's bar area from 1 row to 4 rows using a mirrored
   waveform style (2 rows above an implicit center line, 2 below,
   mirrored), matching Now Playing's height so the two boxes align.
3. Degrade gracefully back to the current stacked, full-width layout on
   terminals too narrow to fit both boxes side by side.

## Non-goals

- No change to the DSP/analyzer pipeline (`internal/spectrum`), the
  `Player.Spectrum()` interface, the tick loop, or config persistence —
  this is purely a layout and rendering-function change in `internal/ui`.
- No user-configurable layout mode (side-by-side vs. stacked is purely a
  width-driven fallback, not a setting).
- No change to `VisualizerEnabled` semantics — still opt-in via the `v`
  key, still absent from layout entirely when disabled.

## Layout

**Side-by-side (wide terminals):**

`View()` renders Now Playing and the visualizer as one horizontal group
when `cfg.VisualizerEnabled` and there's enough remaining width:

```
renderNowPlaying (auto-width, unchanged)  |  renderVisualizerBox (remaining width)
Channels box | Tunes box                              (unchanged)
footer
```

- Now Playing keeps its existing auto-width-to-content sizing — untouched.
- Visualizer width = `m.width - lipgloss.Width(nowPlayingRendered) - decorationPerBox`.
- Visualizer height is fixed at 4 content rows regardless of width, so it
  matches Now Playing's 4 content rows and the two boxes' borders align
  when joined with `lipgloss.JoinHorizontal(lipgloss.Top, ...)`.

**Stacked fallback (narrow terminals):**

If the remaining width after Now Playing falls below a floor —
`minDisplayBars + 2` columns (10), matching the existing `minDisplayBars`
degradation floor used elsewhere in `visualizer.go` — fall back to today's
layout: Now Playing full row, visualizer full-width row below it via the
existing `fullBoxWidth()` path, 4 rows tall (same rendering function, just
given full terminal width instead of the leftover width).

This means `renderVisualizerBox` always renders 4 rows; only its *width*
and *horizontal position* change between the two layouts. When
`VisualizerEnabled` is false, layout is exactly as it is today — Now
Playing alone on its row, no width recalculation needed.

## Rendering: mirrored waveform bars

Each bar column represents one (resampled) frequency band value `v` in
`[0,1]`, same as today. Instead of one character encoding `v` via 8
sub-cell levels (`▁▂▃▄▅▆▇█`) in a single row, it's now spread across 2 rows
on each side of an implicit center line (4 rows total, no visible
center-line glyph — the split between the inner-top and inner-bottom rows
*is* the center):

- Inner row (touching center): fills first. `v` in `[0, 0.5]` maps to this
  row's 8 sub-levels via `barChar(v*2)`.
- Outer row: fills once the inner row is maxed. `v` in `[0.5, 1.0]` maps to
  this row's 8 sub-levels via `barChar((v-0.5)*2)`; renders as a space
  (not `▁`) while `v <= 0.5`, so quiet bands read as empty rather than
  showing a flat baseline in the outer row.
- The row below center mirrors the row above center exactly — same
  characters, same colors — computed once per column and reused for both
  passes.
- `barChar()` and `gradientColor()` (existing functions in `visualizer.go`)
  are reused unchanged, just invoked per-row instead of once per column.
- `resampleBands`/`displayBarCount` are unchanged in behavior, but now
  receive the visualizer's actual rendered width (leftover width in the
  side-by-side case, `fullBoxWidth()` in the stacked case) rather than
  always `fullBoxWidth()`.

Silence/idle behavior (nil bands) is unchanged in spirit: renders as empty
bars (all-space cells), not a collapsed/hidden box.

## Testing

- `internal/ui`: unit tests for the new width-threshold fallback logic
  (side-by-side above the floor, stacked at/below it) using synthetic
  `m.width` values, following the existing `boxWidth`/`fullBoxWidth` test
  patterns.
- `internal/ui`: unit tests for the mirrored-row character/color
  computation (e.g. `v=0.3` → inner row filled to a specific glyph, outer
  row blank; `v=0.8` → inner row maxed, outer row partially filled),
  extending the existing `visualizer_test.go` coverage of `barChar`/
  `resampleBands`.
- Visual appearance (actual alignment, border-joining, terminal rendering)
  is not unit-testable — per CLAUDE.md, requires manual verification via
  `go run .` in a real terminal.
