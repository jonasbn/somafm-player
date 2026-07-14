# somafm-player

Go TUI (Bubble Tea) for playing SomaFM Icecast streams.

## Gotchas

- **Test isolation:** any test that exercises the quit-key path or calls `config.Save`/`config.Load` MUST set `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` first — otherwise it silently overwrites the real `~/.config/somafm-player/config.json` on the dev machine. This has actually happened once; see `internal/config/config_test.go` for the pattern to copy.
- **Run `go mod tidy` after adding any new direct import.** `go get` alone leaves the new package marked `// indirect` in go.mod until a real import references it and tidy runs; this was caught by review twice (bubbletea, lipgloss).
- **`github.com/romantomjak/shoutcast` logs to stderr via the standard `log` package** (connection open/close, HTTP headers, every metadata update). `main.go` calls `log.SetOutput(io.Discard)` before starting the audio pipeline to stop this from corrupting the Bubble Tea TUI — don't remove that line.
- **`internal/player/real.go` uses oto v1**, which has a process-wide singleton `oto.Context` (creating a second one while the first is open panics). `Play()` serializes teardown of the previous stream via a `done`-channel join before ever starting a new one — treat that logic as load-bearing, not incidental, if touching this file.
- **No TTY/audio hardware in typical sandboxed agent environments** — interactive TUI behavior and actual audio playback can only be verified by a human running `go run .` in a real terminal, not by an agent.
- **IDE diagnostics shown after an edit can be stale** (a pre-edit snapshot), especially after a subagent commits — they've repeatedly shown phantom `undefined`/`missing method` errors for code that already builds and tests cleanly. Don't trust the diagnostics panel over `go build ./...`/`go test ./...`; verify directly before reacting to a reported error.
- **`internal/spectrum.Analyzer.Feed()` copies its input before a non-blocking channel send** — the caller (`real.go`'s read loop) reuses its buffer, and the send must never block the audio path. Treat that copy and the `select`/`default` as load-bearing, not incidental, if touching `internal/spectrum`.
