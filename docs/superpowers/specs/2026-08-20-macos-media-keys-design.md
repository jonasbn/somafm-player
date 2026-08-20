# Design: macOS media key (play/pause) integration

Date: 2026-08-20

## Problem

The user has a play/pause key on their macOS keyboard that already
controls Spotify, Apple Music, and YouTube in the browser, but has no
effect on somafm-player. macOS routes hardware media keys (since
10.12.2) to whichever app most recently registered itself with the
system's `MPRemoteCommandCenter` / `MPNowPlayingInfoCenter` APIs
(`MediaPlayer.framework`) — the old direct `NSEvent`/`CGEventTap`
media-key capture is deprecated and unreliable. There is no existing Go
library wrapping this framework, so it requires a small cgo/Objective-C
bridge.

This is feasible without an app bundle or special entitlements: a prior
art example, [mpd-now-playable](https://github.com/00dani/mpd-now-playable),
does exactly this for a headless MPD client as a plain launchd
background daemon (PyObjC bridge to the same framework, no Accessibility
permission, no bundle). somafm-player already builds darwin-only with
`CGO_ENABLED=1` (`.goreleaser.yaml`), so this fits the project's existing
build shape rather than introducing a new one.

somafm-player currently has no "pause" concept at all: `player.Player`
exposes `Play(streamURL)`, `Stop()` (unused by any key today), and
`SetMuted`/`SetVolume`. The only way to affect playback via the keyboard
today is `enter` (start a channel) and `m` (mute toggle).

## Goals

1. Pressing the hardware Play/Pause key toggles mute, identically to
   pressing `m` today.
2. Control Center, the menu-bar Now Playing widget, and the lock screen
   show the current channel/track while somafm-player is playing, and
   playback state (playing/paused) reflects mute state.
3. The app degrades gracefully if media-key registration fails or is
   unavailable — never a fatal error, never a hang.
4. `go build ./...` / `go vet ./...` / tests stay green on any OS, even
   though the feature itself only exists on darwin.

## Non-goals

- No true pause/resume of the live stream (no buffering-and-resuming a
  live Icecast connection) — this is explicitly out of scope; "pause"
  means "mute," matching the existing `m` key behavior exactly.
- No hardware Next/Previous-track key handling. `MPRemoteCommandCenter`
  offers `nextTrackCommand`/`previousTrackCommand`, but those commands
  are not registered, so macOS won't route them here — they fall
  through to whatever app is next in line.
- No "resume last channel" behavior. The hardware key always toggles
  mute — including before any channel has ever been played, same as
  pressing `m` — there is no existing "resume" concept in the app today
  and this doesn't introduce one. (The play/pause/toggle commands are
  registered and enabled unconditionally in `mediakeys_start`, not
  gated on playback state; what *is* gated is Now Playing metadata
  publication — `syncNowPlaying` skips it entirely until a channel has
  been played, so macOS never shows a blank Now Playing item.)
- No change to `player.Player`, `internal/spectrum`, or the DSP/audio
  pipeline in `internal/player/real.go`.

## Architecture

A new `internal/mediakeys` package, split by build tag:

- `//go:build darwin` — the real implementation: a small Objective-C
  shim (`bridge.m`/`bridge.h`) linked via cgo against
  `MediaPlayer.framework` and `Foundation.framework`. It registers
  `MPRemoteCommandCenter` play/pause/toggle-play-pause handlers and runs
  a dedicated goroutine pumping a `CFRunLoop`, locked to its OS thread
  via `runtime.LockOSThread()` — required for the ObjC callbacks to fire
  at all (the same run-loop requirement the PyObjC precedent hit).
- `//go:build !darwin` — a no-op stub satisfying the same `Controller`
  interface, so the package compiles and behaves inertly on any other
  OS. Not required by CI (which runs on `macos-latest`) but keeps the
  module portable for any contributor building elsewhere.

Events cross the cgo boundary into Go over a small buffered channel,
following the same shape as `player.Player.Messages() <-chan Msg`. The
send from the ObjC callback into the channel is non-blocking (buffered,
size 1, drop-if-full), mirroring the load-bearing non-blocking-send
pattern already used in `internal/spectrum.Analyzer.Feed()` — a media
key callback must never block on Go being slow to receive.

## Components

- **`internal/mediakeys/mediakeys.go`** (package-level, all platforms) —
  defines the public surface:
  ```go
  type Event int
  const PlayPauseEvent Event = iota

  type NowPlayingInfo struct {
      Channel string
      Title   string
      Artist  string
  }

  type Controller interface {
      Events() <-chan Event
      SetNowPlaying(info NowPlayingInfo)
      SetPlaying(playing bool)
      Close()
  }
  ```
- **`internal/mediakeys/darwin.go`** (`//go:build darwin`) — the real
  `Controller`: constructs the ObjC bridge, starts the run-loop
  goroutine, implements `SetNowPlaying`/`SetPlaying` as cgo calls that
  update the `MPNowPlayingInfoCenter` info dictionary and playback
  state, and `Close()` to unregister handlers and stop the run loop.
  `New() (Controller, error)` returns an error if bridge/registration
  setup fails at runtime.
- **`internal/mediakeys/bridge.h` / `bridge.m`** — the Objective-C shim:
  C functions cgo calls into (`mediakeys_start`, `mediakeys_stop`,
  `mediakeys_set_now_playing`, `mediakeys_set_playing`), registering
  `MPRemoteCommandCenter.shared().{playCommand,pauseCommand,
  togglePlayPauseCommand}` handlers that call back into an `//export`ed
  Go trampoline function.
- **`internal/mediakeys/other.go`** (`//go:build !darwin`) — no-op
  `Controller`: `Events()` returns a channel that's never written to;
  `New()` always succeeds; `SetNowPlaying`/`SetPlaying`/`Close` are
  no-ops.
- **`internal/mediakeys/fake.go`** — `FakeController` for tests,
  mirroring the existing `player.FakePlayer` pattern: exposes its event
  channel for tests to push synthetic `Event`s onto, and records the
  last `NowPlayingInfo`/playing state it was given.
- **`internal/ui/media_keys.go`** (new) — `waitForMediaKeyCmd(c
  mediakeys.Controller) tea.Cmd`, same shape as the existing
  `waitForPlayerMsg`; an `Update` case for the resulting message that
  calls the existing `m.toggleMute()`.
- **`ui.Model`** gains a `mediaKeys mediakeys.Controller` field, passed
  into `ui.New(cfg, chs, player, hist, mediaKeys)` alongside the
  existing `player.Player` and `history.History` arguments.
- **`main.go`** constructs the controller via `mediakeys.New()` right
  next to the existing `player.NewRealPlayer()` call. On error, falls
  back to a no-op controller (same type used for non-darwin builds)
  rather than treating it as fatal.

## Data flow

1. `main.go` builds the `mediakeys.Controller` and passes it into
   `ui.New`.
2. `Model.Init()` batches `waitForMediaKeyCmd(m.mediaKeys)` into its
   initial `tea.Cmd`s, alongside `waitForPlayerMsg`, `tickCmd`, etc.
3. Hardware key press → macOS invokes the registered ObjC command
   handler (only happens while this app is the current "Now Playing"
   app) → handler does a non-blocking send on the events channel → the
   goroutine blocked in `Events()` unblocks → `waitForMediaKeyCmd`'s
   `tea.Cmd` returns a message → `Update()` calls `m.toggleMute()`
   (identical effect to pressing `m`) and re-issues
   `waitForMediaKeyCmd` to keep listening.
4. Separately, whenever `m.nowPlaying` changes (new channel resolved,
   track changed, connection lost/regained — the existing cases in
   `handlePlaybackMsg`) or `toggleMute()` runs, the same code paths also
   call `m.mediaKeys.SetNowPlaying(...)` and `m.mediaKeys.SetPlaying(...)`
   so Control Center/lock screen mirror actual app state. These are
   synchronous, cheap cgo calls (setting an `NSDictionary` and a
   playback-state enum) — not on any audio-critical path.

## Error handling

- `mediakeys.New()` can fail at runtime (rare — e.g. framework
  registration failing for an unforeseen reason). `main.go` treats this
  as non-fatal: log it (suppressed like everything else via the
  existing `log.SetOutput(io.Discard)`) and continue with a no-op
  controller. The player must never fail to start because media-key
  registration failed.
- No explicit cleanup ordering is required on quit — `Close()` is
  provided for hygiene (unregisters command handlers, stops the run
  loop) but the process exit already reclaims the goroutine/OS thread
  when `main()` returns after `tea.Quit`.

## Testing

- **Unit-testable** (via `FakeController`, mirroring `FakePlayer`):
  `Update()`'s handling of a media-key event (toggles mute, matches
  what pressing `m` does), and that `SetNowPlaying`/`SetPlaying` are
  called with the right values when `nowPlaying` changes or mute is
  toggled.
- **Not unit-testable, requires manual verification**: the cgo/ObjC
  bridge itself — whether macOS actually routes the hardware key here,
  whether Control Center shows the right metadata. Per the existing
  CLAUDE.md gotcha (no TTY/audio hardware in sandboxed agent
  environments), this must be verified by a human running `go run .` on
  a real Mac and pressing the physical key.
- CI already runs on `macos-latest`
  (`.github/workflows/pr-main-build-test-lint.yml`), so `go build`,
  `go vet`, and lint will catch real cgo/ObjC compile errors even though
  the interactive behavior itself can't be asserted there.

## Resources

- [MPRemoteCommandCenter — Apple Developer Documentation](https://developer.apple.com/documentation/mediaplayer/mpremotecommandcenter)
- [MPNowPlayingInfoCenter — Apple Developer Documentation](https://developer.apple.com/documentation/mediaplayer/mpnowplayinginfocenter)
- [00dani/mpd-now-playable](https://github.com/00dani/mpd-now-playable) — prior art: a non-GUI background daemon using this same mechanism, PyObjC bridge, no app bundle.
- [Writing Apps in Go and Swift — Young Dynasty](https://youngdynasty.net/posts/writing-mac-apps-in-go/) — background on Go/cgo/Cocoa framework interop patterns.
