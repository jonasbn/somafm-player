# TODO

## Release platforms

- Start release binaries with macOS (amd64 + arm64) only.
- Fast-follow: add Linux (amd64 + arm64) once macOS release pipeline is proven —
  needs a native `ubuntu-latest` build leg since `oto` v1 uses cgo/ALSA on Linux.
- Fast-follow: add Windows (amd64) — `oto` v1's Windows backend is pure Go
  (syscalls to `winmm.dll`, no cgo), so this can cross-compile from any runner
  once there's demand for it.
- Fast-follow: sign & notarize macOS binaries once there's an Apple Developer
  account to do it with (currently ship unsigned; README documents the
  Gatekeeper workaround).
- Fast-follow: publish a Homebrew tap (`jonasbn/homebrew-tap`) once the
  GoReleaser pipeline is proven out, for `brew install jonasbn/tap/somafm-player`.
