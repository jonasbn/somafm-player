# SomaFM TUI Player — Design

## Overview

A terminal user interface, written in Go with Bubble Tea, for listening to SomaFM Icecast
streams. Fetches the live channel list, plays the highest-quality MP3 stream via a pure-Go
pipeline (shoutcast → go-mp3 → oto), and supports channel switching, bookmarking (channels and
individual tunes), session history, volume control, and six selectable color themes.

## Stack

- Go
- Bubble Tea (Elm-architecture TUI framework) + Lip Gloss (styling) + Bubbles (list components)
- `github.com/romantomjak/shoutcast` — ICY metadata + stream reader
- `github.com/hajimehoshi/go-mp3` — MP3 decoding
- `github.com/hajimehoshi/oto` — PCM playback
- SomaFM's `https://somafm.com/channels.json` — live channel directory

AAC variants are explicitly out of scope for v1 (see "Stream Variant Selection" below) to avoid
an external ffmpeg/faad2 dependency and stay pure-Go.

## Architecture & Components

A single root Bubble Tea `Model` composed of sub-models, each with a narrow responsibility:

- **`player`** — owns the shoutcast → go-mp3 → oto pipeline as a background goroutine. Emits
  `tea.Msg` values back into the Update loop over a channel: `trackChangedMsg`,
  `connectionLostMsg`, `reconnectedMsg`. Wraps the connection in a retry loop with backoff
  (1s, 2s, 5s, 5s, 5s...) and retries indefinitely — no hard failure cap, since transient drops
  are common for internet radio and the user can switch channels or quit if a stream seems
  actually dead.
- **`channels`** — fetches and parses `channels.json` on startup; holds the channel list and
  currently selected index. On fetch failure, surfaces an inline error state in the Channels
  panel ("Couldn't load channel list — check your connection, press `r` to retry") instead of
  crashing.
- **`bookmarks`** — two independent lists: bookmarked **channels** (quick-access shortcuts) and
  bookmarked **tunes** (starred track+artist+channel+timestamp entries — a discovery log, since a
  live stream's past track can't be replayed). Loaded from and saved to the config file.
- **`history`** — an in-memory ring buffer of the last 5 tracks played this session
  (`{title, artist, channel, playedAt}`), global across channel switches. Session-only, never
  persisted, cleared on exit — per the feature requirement.
- **`theme`** — a `map[string]Theme` of Lip Gloss style sets (background/foreground/accent/
  border/muted-text) for Nord, Dracula, Gruvbox, Tokyo Night, Solarized Dark, and Solarized Light.
  Swapping themes swaps which style set the View functions read from. Lip Gloss's color-profile
  detection handles terminals with limited color support.
- **`config`** — loads/saves JSON at `~/.config/somafm-player/config.json` (respecting
  `$XDG_CONFIG_HOME`). Saved on every mutating action (bookmark toggle, volume/mute change, theme
  change, channel switch) since the file is small and this avoids losing state on a crash/kill.

Panel switching (Channels / Bookmarked Channels / Bookmarked Tunes / History) is a `viewMode`
enum on the root model. All four modes render through the same list component, fed different
backing data.

## Stream Variant Selection

Each SomaFM channel typically publishes multiple variants (bitrate/codec). The player always
selects the highest-quality **MP3** variant automatically — no per-channel user choice. This keeps
the whole pipeline pure-Go (no ffmpeg/faad2 subprocess for AAC) and removes a layer of UI/state
that would otherwise be needed to expose variant selection.

## Layout

Single view, stacked panels — no full-screen tabs, no popups:

```
┌─ Now Playing ──────────────────────────────────────┐
│ ♪ Track Title — Artist                              │
│ Channel: Groove Salad   •   128k MP3   •  Reconnecting…(when applicable) │
│ Elapsed: 1:32   Session: 42:10                       │
│ Vol: ████████░░ 80%  [muted icon if muted]           │
└──────────────────────────────────────────────────────┘
┌─ Channels ▸ Bookmarked Channels ▸ Bookmarked Tunes ▸ History ┐
│ > Groove Salad          Ambient/Downtempo             │
│   Drone Zone            Slow Ambient       ★ bookmarked│
│   Deep Space One        ...                            │
└──────────────────────────────────────────────────────┘
 [Theme: Nord]                       tab focus · j/k move · enter play · b bookmark · c/f/s/H panels · +/- vol · m mute · t theme · q quit
```

The focused panel (Now Playing or the list panel) is indicated with a highlighted border using
the active theme's accent color; `tab` toggles focus between them.

### Time display

Icecast/SHOUTcast streams don't broadcast a track's total duration — only the ICY `StreamTitle`
on change. There is no seek bar. Instead:
- **Elapsed**: time since the current track's `StreamTitle` last changed, reset to 0:00 on each
  metadata update.
- **Session**: total time connected/listening this session.

## Keybindings (vim-style + mnemonics)

| Key | Action |
|---|---|
| `tab` | toggle focus between Now Playing and the list panel |
| `j`/`k` or `↓`/`↑` | move selection within the focused list panel |
| `enter` | play selected channel (Channels / Bookmarked Channels panels only) |
| `c` | switch list panel to Channels |
| `f` | switch list panel to Bookmarked Channels ("favorites") |
| `s` | switch list panel to Bookmarked Tunes |
| `H` | switch list panel to History |
| `b` | context-sensitive bookmark toggle (see below) |
| `+`/`-` or `←`/`→` | volume down/up by 5% |
| `m` | toggle mute |
| `t` | cycle theme (Nord → Dracula → Gruvbox → Tokyo Night → Solarized Dark → Solarized Light → Nord) |
| `q` / `ctrl+c` | quit |

### `b` context rules

- Now Playing focused → bookmark/unbookmark the currently playing **tune**.
- List panel focused, showing Channels or Bookmarked Channels → bookmark/unbookmark the
  **selected channel**.
- List panel focused, showing History or Bookmarked Tunes → bookmark the **selected historical
  entry** into Bookmarked Tunes (lets you retroactively star something from history, not only the
  currently-playing track).

## Data & Persistence

Config file, created on first run, saved on every mutating action:

```json
{
  "lastChannel": "Groove Salad",
  "volume": 80,
  "muted": false,
  "theme": "Nord",
  "bookmarkedChannels": ["Groove Salad", "Drone Zone"],
  "bookmarkedTunes": [
    {"title": "...", "artist": "...", "channel": "Drone Zone", "bookmarkedAt": "2026-07-13T10:22:00Z"}
  ]
}
```

Location: `~/.config/somafm-player/config.json`, respecting `$XDG_CONFIG_HOME`.

History is **not** part of this file — it's an in-memory-only ring buffer, per the feature
requirement that history doesn't need to survive a restart.

## Startup Behavior

On launch: load config, fetch `channels.json`, then auto-resume playback of `lastChannel` if
present (falls back to the Channels panel with no auto-play if there's no last channel, or if the
channel fetch failed).

## Reconnection Behavior

On stream disconnect, the player goroutine retries with backoff (1s, 2s, 5s, 5s, 5s...)
indefinitely. While disconnected, Now Playing swaps the track title display for "Reconnecting…"
(keeping the last known track title dimmed underneath); on success it's replaced with the normal
live display. There is no retry cap or hard error state — silent, patient auto-retry is preferred
over an intrusive error for what is typically a transient blip.

## Theming

Six themes (Nord, Dracula, Gruvbox, Tokyo Night, Solarized Dark, Solarized Light), each a Lip
Gloss style set covering background, foreground, accent, border, and muted-text colors. Cycled
with `t`; the active theme name is persisted in config and restored on next launch.

## Testing Approach

- Unit tests for pure logic: `channels.json` parsing, config load/save round-trip, history
  ring-buffer behavior, bookmark toggle logic (including the context-sensitive `b` rules), theme
  cycling.
- The Bubble Tea `Update` function is tested by feeding it messages and asserting resulting model
  state — no real terminal or audio required.
- The player/audio layer (shoutcast/decoder/oto wiring) is the one part that resists meaningful
  unit testing; it sits behind a small interface so it can be stubbed for Update-layer tests, and
  is validated manually against the real stream.

## Out of Scope (v1)

- AAC/AAC-HE stream variants (would require an external ffmpeg/faad2 dependency).
- Per-channel bitrate/codec selection by the user.
- Persisting history across restarts.
- Fully configurable/remappable keybindings.
