# Design: CI, security linting, and release automation

Date: 2026-07-15

## Problem

The project has no GitHub Actions workflows at all: no automated build/test
on push or PR, no dependency or workflow-security scanning, and no way for
someone to download a `somafm-player` binary without cloning the repo and
running `go build`. We want a baseline CI + release setup appropriate for a
small Go CLI tool, including `zizmor` to keep the workflow YAML itself free
of common GitHub Actions security mistakes (unpinned actions, script
injection via untrusted `${{ }}` expansion, excess `permissions:`).

## Goals

1. Every push/PR to `main` runs a build, the test suite, `golangci-lint`,
   and `govulncheck`.
2. Every `v*.*.*` tag push builds and publishes downloadable macOS binaries
   (amd64 + arm64) to a GitHub Release via GoReleaser, with checksums and
   auto-generated release notes.
3. The workflow files themselves are linted by `zizmor` for security issues,
   both on every change to them and on a recurring schedule (so newly
   published advisories get caught even without a workflow edit).
4. Dependencies (Go modules and Actions versions) get automated update PRs
   via Dependabot.
5. File names follow the SSW `{{TRIGGER}}-{{ACTIONS}}-{{ADDITIONAL
   DETAILS}}.yml` convention (https://www.ssw.com.au/rules/workflow-naming-scheme).

## Non-goals

- Linux and Windows release binaries — noted as fast-follows in
  `docs/TODO.md`. Not built now because `oto` v1 needs cgo/ALSA on Linux,
  which means a real native-runner build leg, not a quick addition.
- Code signing / notarization for macOS binaries — ship unsigned for now;
  README gets a short Gatekeeper workaround note. Fast-follow in
  `docs/TODO.md`.
- A Homebrew tap — fast-follow in `docs/TODO.md`, once the release pipeline
  itself is proven out.
- CodeQL — skipped; low value for a small TUI with no network-facing attack
  surface beyond parsing SomaFM's own `channels.json`/`.pls` data.
- No change to `CHANGELOG.md`'s hand-maintained format — GoReleaser's
  auto-generated release notes supplement it, they don't replace it.

## Workflow files

| File | Trigger(s) | Purpose |
|---|---|---|
| `.github/workflows/pr-main-build-test-lint.yml` | `pull_request` (any branch → `main`) and `push` to `main` | `go build ./...`, `go test ./...`, `golangci-lint run`, `govulncheck ./...` as separate jobs in one workflow |
| `.github/workflows/main-schedule-zizmor-lint.yml` | `push` touching `.github/workflows/**`, plus a weekly `schedule` (cron) | Runs `zizmor` against `.github/workflows/` |
| `.github/workflows/tag-release-goreleaser.yml` | `push` of tags matching `v*.*.*` | Runs GoReleaser to build, package, and publish the release |
| `.github/dependabot.yml` | n/a (config, not a workflow) | Weekly update PRs for the `gomod` and `github-actions` ecosystems |

## `pr-main-build-test-lint.yml`

Four jobs, all on `runs-on: macos-latest` (matches the only supported target
platform, and avoids any Linux-vs-macOS behavioral drift in test output):

- `build`: `go build ./...`
- `test`: `go test ./...`
- `lint`: `golangci-lint run` (covers `gofmt`, `go vet`, `staticcheck`, and
  more in one pass — the standard aggregator for Go)
- `govulncheck`: `govulncheck ./...`, using the official `golang/govulncheck-action`

Jobs run in parallel (no `needs:` between them) since none depend on each
other's output — this keeps PR feedback fast.

## `main-schedule-zizmor-lint.yml`

Single job running `zizmor` (via `uvx zizmor` or the community Action,
whichever has better maintenance at implementation time) against
`.github/workflows/`, failing the run on any medium+ severity finding.
Triggers:

- `push` with a `paths:` filter on `.github/workflows/**`, so it re-runs
  whenever a workflow file changes.
- `schedule: cron` weekly, so it also re-flags workflows against newly
  published `zizmor` audit rules even when nothing here has changed.

## `tag-release-goreleaser.yml`

> **Revised during implementation (2026-07-15):** the partial/split build
> approach described below assumed GoReleaser OSS could merge builds from
> two runners. It can't — that's a GoReleaser Pro-only feature (confirmed
> against https://goreleaser.com/customization/partial/). Task 4's local
> snapshot build had already shown cgo cross-compilation from an arm64 Mac
> to darwin/amd64 works natively, so the actual implementation runs a
> single `macos-latest` job (arm64; `macos-14` is itself now deprecated
> per actions/runner-images) with plain `goreleaser release --clean`, building
> both arches from `.goreleaser.yaml`'s existing `builds:` list. The
> Release notes/changelog behavior described below is unaffected — it's
> exactly what a normal `goreleaser release` run produces.

Triggered on `v*.*.*` tag pushes (matching the existing `v0.1.0`–`v0.3.2`
tagging already in use). ~~Because `oto` v1 requires cgo on macOS (CoreAudio
bindings), GoReleaser can't cross-compile arm64↔amd64 from a single runner.
Uses GoReleaser's **partial/split build** pattern~~ (superseded, see note
above):

1. ~~A `macos-13` (Intel) job builds the `darwin/amd64` leg:
   `goreleaser build --single-target --snapshot` (or `release --split`,
   depending on the GoReleaser version at implementation time) with
   `CGO_ENABLED=1`, uploading the partial dist as a build artifact.~~
2. ~~A `macos-14` (Apple Silicon) job builds the `darwin/arm64` leg the same
   way.~~
3. ~~A final `merge` job downloads both partial dists and runs
   `goreleaser continue --merge` (or equivalent) to assemble one GitHub
   Release: `.tar.gz` archive per arch, a `checksums.txt`, and release notes.~~

Release notes: GoReleaser's changelog groups commits since the last tag into
Features/Fixes/Other (matching the `feat:`/`fix:`/etc. prefixes already used
in commit messages), with a fixed trailer line pointing readers to
`CHANGELOG.md` for the curated version.

`.goreleaser.yaml` at the repo root configures: two builds (darwin/amd64,
darwin/arm64) with `CGO_ENABLED: "1"`, archive format `tar.gz`, checksum
file, and the changelog/release-notes settings above.

## `.github/dependabot.yml`

Two `updates:` entries:

- `package-ecosystem: gomod`, weekly
- `package-ecosystem: github-actions`, weekly

## Testing / verification

- Workflow YAML has no unit tests; verification is: push a branch and watch
  `pr-main-build-test-lint.yml` and `main-schedule-zizmor-lint.yml` run
  successfully in the Actions tab, then push a test tag (e.g. `v0.3.3-rc1`)
  to a throwaway branch/fork or run `goreleaser release --snapshot
  --clean` locally to validate `tag-release-goreleaser.yml`/
  `.goreleaser.yaml` without actually publishing a release.
- `zizmor` itself should be run locally against the two other new workflow
  files before merging, so any finding is caught before the scheduled job
  ever runs.
