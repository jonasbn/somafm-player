# Design: ASCII logo banner

Date: 2026-07-14

## Problem

`docs/TODO.md` asks for an ASCII-art banner above the player/visualizer row:
a default "somafm" logo, plus channel-specific variants for Drone Zone
(and "Drone Zone 2") and Deep Space One, with the three art blocks already
specified in the TODO file (Big font for the default, Standard/wavescape
for Drone Zone, Rectangles/Modular for Deep Space One).

## Goals

1. Render a banner above Now Playing showing the default "somafm" logo
   normally, switching to a channel-specific logo when that channel is the
   one currently playing.
2. Degrade gracefully on terminals too narrow for the active logo's width.
3. Color the banner using the active theme so it stays consistent with the
   rest of the themed UI (Nord, Dracula, Gruvbox, Tokyo Night, Solarized
   Dark/Light) rather than a single hardcoded color scheme.

## Non-goals

- No toggle/keybinding/config flag for the banner — it's always rendered,
  matching the TODO's plain "should be above the player" wording. (Contrast
  with the visualizer, which is opt-in via `v` + `VisualizerEnabled`.)
- No new channels beyond what the TODO specifies. Any channel title other
  than "Drone Zone", "Drone Zone 2", or "Deep Space One" — including no
  channel playing yet — uses the default logo.
- No change to `internal/channels`, `internal/player`, config schema, or
  the tick/message loop.

## Architecture

New package `internal/logo`, mirroring how `internal/theme` is a small,
self-contained lookup consumed by `internal/ui`:

```go
package logo

// For returns the ASCII art lines for the given channel title, falling
// back to the default "somafm" logo for any unmatched title (including "").
func For(channelTitle string) (lines []string, isDefault bool)
```

Internally, three raw string constants hold the art (copied verbatim from
`docs/TODO.md`), and a lookup table maps channel title → art:

```go
var byChannel = map[string]string{
    "Drone Zone":    droneZoneArt,
    "Drone Zone 2":  droneZoneArt,
    "Deep Space One": deepSpaceOneArt,
}
```

`isDefault` tells the caller which color to use (see Rendering) without
`internal/ui` needing to know the art content or the channel-name mapping.

## Rendering

`internal/ui/view.go` gains `renderLogo(t theme.Theme) string`:

1. Call `logo.For(m.nowPlaying.channel)`.
2. Pick the color: `t.Hot` when `isDefault` (the default "somafm" banner
   reads as red/warm in every theme, since each theme's `Hot` color is
   already a red-family accent — e.g. Dracula `#FF5555`, Nord `#BF616A`),
   otherwise `t.Accent` (the same color used for focused-panel borders and
   the visualizer's mid-range — the app's one existing "emphasis" color).
3. Compute the widest line's rune length. If `m.width` is less than that
   width, render a single-line fallback styled with the same color:
   `"SomaFM"` — always this literal text, not the channel name, since the
   Now Playing box already shows the channel via `Channel: %s`.
4. Otherwise join the art lines with `\n` and wrap in
   `lipgloss.NewStyle().Foreground(color)`.

`View()` prepends this banner to `sections`, above `renderNowPlayingRow`:

```go
sections := []string{m.renderLogo(t), m.renderNowPlayingRow(t), lists}
```

No border, no padding — plain banner text, not another bordered panel like
Now Playing/Channels/Tunes.

## Testing

- `internal/logo/logo_test.go`: table test asserting `For("Drone Zone")`
  and `For("Drone Zone 2")` both return the Drone Zone art with
  `isDefault=false`, `For("Deep Space One")` returns its art with
  `isDefault=false`, and `For("")`/`For("Groove Salad")`/any unmatched
  title returns the default art with `isDefault=true`.
- `internal/ui`: a narrow-width test (`m.width` below the active logo's
  widest line) asserting `renderLogo` returns the one-line `"SomaFM"`
  fallback rather than multi-line art, and a normal-width test asserting
  the full art (correct line count) renders when there's room, following
  the existing width-threshold test pattern used for the visualizer's
  side-by-side/stacked fallback.
- Actual color/visual appearance in a real terminal is not unit-testable —
  per CLAUDE.md, requires manual verification via `go run .`.
