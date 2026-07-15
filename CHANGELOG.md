# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.1] - 2026-07-15

### Fixed

- CI `lint` job no longer crashes on startup: `golangci-lint`'s prebuilt
  binary lagged behind the module's declared Go version, so it now builds
  from source with the toolchain already provisioned for the job, and
  `go.mod`'s `go` directive was retidied to the version actually required
  by dependencies.
- Addressed unchecked-error-return (`errcheck`) findings surfaced now that
  lint runs to completion: stream/response/oto `Close()` calls and test
  `Write()` calls now explicitly discard their errors.

## [0.4.0] - 2026-07-15

### Added

- CI workflow (`pr-main-build-test-lint`) running build, test, and lint on
  pull requests and pushes to main.
- Scheduled `zizmor` workflow lint check on main.
- GoReleaser configuration and `tag-release-goreleaser` workflow, building and
  publishing darwin amd64/arm64 release binaries on tag push.
- Dependabot configuration for Go modules and GitHub Actions.
- Installation instructions in the README, including a Gatekeeper workaround
  for unsigned macOS release binaries.

## [0.3.2] - 2026-07-15

### Fixed

- Screen is now cleared via the alt-screen buffer before drawing the TUI, so no
  leftover terminal artifacts show through on start.

## [0.3.1] - 2026-07-15

### Added

- ASCII logo banner rendered above Now Playing, with channel-aware artwork lookup.
- Visualizer moved beside Now Playing and redrawn as a 4-row mirrored waveform,
  with a stacked fallback layout for narrow terminals.
- README rewritten as a feature/usage front page, including a VHS demo recording.
- LICENSE file added.

### Fixed

- Visualizer bar width now accounts for the panel's padding budget.
- Visualizer glyphs are top-anchored so the waveform actually mirrors correctly.
- A truncated line in the default ASCII art was restored, with a regression test added.
- Spectrum bands that never receive an FFT bin are now interpolated instead of
  rendering blank.

## [0.3.0] - 2026-07-13

### Added

- Live equalizer/spectrum visualizer: FFT analysis pipeline with non-blocking
  PCM fan-out, plus DSP helpers for windowing, bucketing, and decay.
- Opt-in `VisualizerEnabled` config setting and a `v` key to toggle it at runtime.
- "Hot" gradient color theme for the equalizer bars.
- Equalizer box rendered in the layout when the visualizer is enabled.

### Fixed

- `resampleBands` guarded against a panic on negative bar counts.
- `bucketMagnitudes` reverted to averaging all bins per bucket, matching the
  intended behavior.

## [0.2.0] - 2026-07-13

### Changed

- Channels and Tunes are now rendered as separate, side-by-side boxes with
  independent focus/navigation state, replacing the earlier combined panel.
- Bookmark handling now branches on the active channels/tunes focus state.

## [0.1.0] - 2026-07-13

Initial tagged release: a working terminal player for SomaFM stations.

### Added

- Bubble Tea TUI scaffold with quit handling.
- Config load/save at the XDG config path, with auto-resume of the last played channel.
- Six-theme color palette with a cycling key.
- SomaFM channel list fetching (`channels.json`), `.pls` playlist resolution,
  and bitrate parsing.
- Real audio playback pipeline (shoutcast + go-mp3 + oto) with reconnect backoff.
- Volume adjustment and mute toggle keys.
- Context-sensitive bookmarking for channels and tunes, with tune deduplication.
- Session-only track history ring buffer.
- Elapsed track time and session time display.
- Now Playing and list panels rendered with theme styling.
- `r` key to retry the channel list fetch after a failure.
- Usage and keybindings section in the docs.

### Fixed

- Silenced the `shoutcast` library's stderr logging, which was corrupting the TUI.
- Made `shoutcast.Open` cancellable and checked the context after a successful open.
- Clamped volume in `renderVolumeBar` to prevent a panic on out-of-range config values.
- Carried over an odd trailing byte across `volumeReader` reads.
- Returned a defensive copy from `History.Entries()` to prevent a slice-leak.
- Synced saved volume/mute settings into the player at construction time.
- Set `PlayedAt` when recording history entries.
- Tidied `go.mod` to correctly mark direct dependencies (bubbletea, lipgloss).
- Isolated `TestUpdate_QuitsOnQ` from the real user config directory.

[0.4.1]: https://github.com/jonasbn/somafm-player/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/jonasbn/somafm-player/compare/v0.3.2...v0.4.0
[0.3.2]: https://github.com/jonasbn/somafm-player/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/jonasbn/somafm-player/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/jonasbn/somafm-player/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/jonasbn/somafm-player/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/jonasbn/somafm-player/releases/tag/v0.1.0
