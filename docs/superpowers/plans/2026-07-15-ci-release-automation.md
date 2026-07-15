# CI, Security Linting, and Release Automation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `somafm-player` a GitHub Actions baseline: PR/main build+test+lint+vuln checks, a zizmor security lint of the workflows themselves, Dependabot updates, and a GoReleaser-driven release pipeline that publishes macOS (amd64+arm64) binaries on tag push.

**Architecture:** Three workflow files plus one Dependabot config, all under `.github/`, named per the SSW `{{TRIGGER}}-{{ACTIONS}}-{{ADDITIONAL DETAILS}}.yml` scheme. Release building uses GoReleaser's split/partial-build pattern across two native macOS runners (Intel + Apple Silicon) because `oto` v1 needs cgo, then a merge job assembles one GitHub Release.

**Tech Stack:** GitHub Actions, `golangci-lint`, `govulncheck`, `zizmor` (via `uvx`), GoReleaser, Dependabot.

## Global Constraints

- Only macOS (darwin/amd64, darwin/arm64) release binaries for now — no Linux/Windows (see `docs/superpowers/specs/2026-07-15-ci-release-automation-design.md` Non-goals).
- macOS binaries ship unsigned/non-notarized — no code signing in this plan.
- No Homebrew tap in this plan.
- No CodeQL.
- Workflow file names are fixed: `pr-main-build-test-lint.yml`, `main-schedule-zizmor-lint.yml`, `tag-release-goreleaser.yml`.
- `CHANGELOG.md` stays hand-maintained; GoReleaser's auto-generated release notes supplement it with a link, they don't replace it.
- Every third-party `uses:` action reference must be pinned to a full commit SHA with a version comment (e.g. `actions/checkout@<sha> # v4.2.2`), not a mutable tag — this is the specific thing `zizmor`'s `unpinned-uses` rule checks for, and the whole reason `main-schedule-zizmor-lint.yml` exists.

---

## Task 1: PR/main build, test, and lint workflow

**Files:**
- Create: `.github/workflows/pr-main-build-test-lint.yml`

**Interfaces:**
- Consumes: nothing (first task, no dependency on other new files).
- Produces: nothing consumed by later tasks — this workflow is self-contained. (Task 2 and Task 5 are independent sibling workflow files, not built on top of this one.)

- [ ] **Step 1: Look up current commit SHAs for the actions this workflow uses**

Run each of these and note the SHA output (use the `gh` CLI, already authenticated in this repo):

```bash
gh api repos/actions/checkout/git/refs/tags/v4.2.2 --jq .object.sha
gh api repos/actions/setup-go/git/refs/tags/v5.1.0 --jq .object.sha
gh api repos/golangci/golangci-lint-action/git/refs/tags/v6.1.1 --jq .object.sha
gh api repos/golang/govulncheck-action/git/refs/tags/v1.0.4 --jq .object.sha
```

If any of these tags no longer exist (the actions may have released newer
versions since this plan was written), use `gh api repos/<owner>/<repo>/tags
--jq '.[0]'` to find the current latest tag and its SHA instead, and use
that version number in the comment.

- [ ] **Step 2: Write the workflow file**

```yaml
name: pr-main-build-test-lint

on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read

jobs:
  build:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@<checkout-sha> # v4.2.2
      - uses: actions/setup-go@<setup-go-sha> # v5.1.0
        with:
          go-version-file: go.mod
      - run: go build ./...

  test:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@<checkout-sha> # v4.2.2
      - uses: actions/setup-go@<setup-go-sha> # v5.1.0
        with:
          go-version-file: go.mod
      - run: go test ./...

  lint:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@<checkout-sha> # v4.2.2
      - uses: actions/setup-go@<setup-go-sha> # v5.1.0
        with:
          go-version-file: go.mod
      - uses: golangci/golangci-lint-action@<golangci-lint-action-sha> # v6.1.1
        with:
          version: latest

  govulncheck:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@<checkout-sha> # v4.2.2
      - uses: actions/setup-go@<setup-go-sha> # v5.1.0
        with:
          go-version-file: go.mod
      - uses: golang/govulncheck-action@<govulncheck-action-sha> # v1.0.4
```

Replace every `<...-sha>` placeholder with the real SHA from Step 1 before
saving — this file must not be committed with placeholder text still in it.

- [ ] **Step 3: Validate the YAML with actionlint**

```bash
go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/pr-main-build-test-lint.yml
```

Expected: no output (no errors). Fix anything actionlint flags (e.g. a
mistyped key) before continuing.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/pr-main-build-test-lint.yml
git commit -m "ci: add pr-main-build-test-lint workflow"
```

Note: this workflow won't actually execute until it's pushed — pushing the
branch (and/or opening a PR) is a shared-visibility action, so confirm with
the user before pushing.

---

## Task 2: Zizmor workflow-security-lint workflow

**Files:**
- Create: `.github/workflows/main-schedule-zizmor-lint.yml`

**Interfaces:**
- Consumes: nothing.
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Look up the current commit SHA for `astral-sh/setup-uv`**

```bash
gh api repos/astral-sh/setup-uv/git/refs/tags/v4.2.0 --jq .object.sha
```

If `v4.2.0` no longer exists, use `gh api repos/astral-sh/setup-uv/tags
--jq '.[0]'` to find the current latest tag/SHA.

- [ ] **Step 2: Write the workflow file**

```yaml
name: main-schedule-zizmor-lint

on:
  push:
    branches: [main]
    paths:
      - '.github/workflows/**'
  schedule:
    - cron: '0 6 * * 1'

permissions:
  contents: read

jobs:
  zizmor:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@<checkout-sha> # v4.2.2 (same SHA looked up in Task 1)
      - uses: astral-sh/setup-uv@<setup-uv-sha> # v4.2.0
      - run: uvx zizmor@latest --min-severity medium .github/workflows/
```

`--min-severity medium` fails the job on medium+ findings while not
blocking on purely informational ones; drop the flag entirely once the repo
is clean and you want maximum strictness.

- [ ] **Step 3: Run zizmor locally first, before relying on CI to catch anything**

```bash
uvx zizmor@latest .github/workflows/
```

Expected: no medium+ severity findings against any of the three workflow
files created in this plan (Task 1's, this one, and Task 5's — run this
again after Task 5 to confirm). If `uvx` isn't available locally, install
`uv` first: see https://docs.astral.sh/uv/getting-started/installation/.

- [ ] **Step 4: Fix any findings, then validate YAML with actionlint**

```bash
go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/main-schedule-zizmor-lint.yml
```

Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/main-schedule-zizmor-lint.yml
git commit -m "ci: add main-schedule-zizmor-lint workflow"
```

---

## Task 3: Dependabot configuration

**Files:**
- Create: `.github/dependabot.yml`

**Interfaces:**
- Consumes: nothing.
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Write the config**

```yaml
version: 2
updates:
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"

  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
```

- [ ] **Step 2: Validate it's well-formed YAML**

```bash
python3 -c "import yaml, sys; yaml.safe_load(open('.github/dependabot.yml'))" && echo OK
```

Expected: `OK`.

- [ ] **Step 3: Commit**

```bash
git add .github/dependabot.yml
git commit -m "chore: add dependabot config for gomod and github-actions"
```

Note: Dependabot itself only actually runs once this is pushed to GitHub and
picked up by the platform — no further local verification is possible.

---

## Task 4: GoReleaser configuration

**Files:**
- Create: `.goreleaser.yaml`

**Interfaces:**
- Consumes: `go.mod` (module path `github.com/jonasbn/somafm-player`, used implicitly by `goreleaser` to name binaries/archives after the module).
- Produces: the `.goreleaser.yaml` config that Task 5's workflow invokes via `goreleaser build` / `goreleaser release`.

- [ ] **Step 1: Install GoReleaser locally for iteration**

```bash
go install github.com/goreleaser/goreleaser/v2@latest
```

- [ ] **Step 2: Write the config**

```yaml
version: 2

before:
  hooks:
    - go mod tidy

builds:
  - id: darwin-amd64
    main: .
    binary: somafm-player
    goos: [darwin]
    goarch: [amd64]
    env:
      - CGO_ENABLED=1

  - id: darwin-arm64
    main: .
    binary: somafm-player
    goos: [darwin]
    goarch: [arm64]
    env:
      - CGO_ENABLED=1

archives:
  - id: default
    formats: [tar.gz]
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}

checksum:
  name_template: "checksums.txt"

changelog:
  use: github
  sort: asc
  groups:
    - title: Features
      regexp: '^.*?feat(\([[:word:]]+\))??!?:.+$'
      order: 0
    - title: Fixes
      regexp: '^.*?fix(\([[:word:]]+\))??!?:.+$'
      order: 1
    - title: Others
      order: 999

release:
  footer: >-
    See [CHANGELOG.md](https://github.com/jonasbn/somafm-player/blob/main/CHANGELOG.md) for the full curated changelog.
```

- [ ] **Step 3: Validate the config**

```bash
goreleaser check
```

Expected: `X configuration is valid` (no errors).

- [ ] **Step 4: Do a local snapshot build to prove it actually builds both arches**

```bash
goreleaser release --snapshot --clean
ls dist/
```

Expected: `dist/` contains a `darwin_amd64` and a `darwin_arm64` build
output, plus `.tar.gz` archives and a `checksums.txt`, with no errors in the
command output. This only works if you're running it on a Mac (cgo build),
matching this plan's macOS-only scope.

- [ ] **Step 5: Commit**

```bash
git add .goreleaser.yaml
git commit -m "build: add goreleaser config for darwin amd64/arm64 releases"
```

---

## Task 5: Tag-triggered release workflow (single runner, both arches)

> **Revised during implementation (2026-07-15):** the original plan called
> for splitting the build across a `macos-13` (amd64) and `macos-14`
> (arm64) runner, then merging with `goreleaser continue --merge`. That
> feature turned out to be **GoReleaser Pro-only** — confirmed against
> https://goreleaser.com/customization/partial/ ("This feature is
> exclusively available with GoReleaser Pro"). `goreleaser release
> --skip=build` (an implementer's first workaround attempt) also isn't a
> valid flag combination in any GoReleaser version (`--skip` doesn't accept
> `build` as a value — confirmed via `goreleaser release --help`).
>
> Task 4's own snapshot build already proved the actual fix: cgo
> cross-compilation from this project's arm64 Mac to `darwin/amd64` works
> natively with no cross-toolchain issues. That means a single runner can
> build both arches, so the split/merge design (and its Pro dependency) is
> unnecessary. This revision runs real `goreleaser release --clean` on one
> `macos-latest` runner (arm64; `macos-14`/`macos-14-arm64` are themselves
> now marked deprecated per actions/runner-images), keeping the original
> grouped-changelog design from Task 4 fully intact — no `gh release
> create` workaround needed.

**Files:**
- Create: `.github/workflows/tag-release-goreleaser.yml`

**Interfaces:**
- Consumes: `.goreleaser.yaml` from Task 4 (must exist and pass `goreleaser check` before this task is meaningful to test).
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Look up current commit SHAs for the actions this workflow uses**

```bash
gh api repos/actions/checkout/git/refs/tags/v4.2.2 --jq .object.sha
gh api repos/actions/setup-go/git/refs/tags/v5.1.0 --jq .object.sha
gh api repos/goreleaser/goreleaser-action/git/refs/tags/v6.1.0 --jq .object.sha
```

As in Task 1, if a tag is gone, look up the current latest tag/SHA instead.
Reuse the checkout/setup-go SHAs already used in Tasks 1/2/5's earlier
attempt if they're still current (`actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2`,
`actions/setup-go@41dfa10bad2bb2ae585af6ee5bb4d7d973ad74ed # v5.1.0`).

- [ ] **Step 2: Write the workflow file**

```yaml
name: tag-release-goreleaser

on:
  push:
    tags:
      - 'v*.*.*'

permissions:
  contents: write

jobs:
  release:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@<checkout-sha> # v4.2.2
        with:
          fetch-depth: 0
      - uses: actions/setup-go@<setup-go-sha> # v5.1.0
        with:
          go-version-file: go.mod
      - uses: goreleaser/goreleaser-action@<goreleaser-action-sha> # v6.1.0
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Replace every `<...-sha>` placeholder with the real SHA from Step 1. One
job, one runner: `.goreleaser.yaml`'s `builds:` section already lists both
`darwin-amd64` and `darwin-arm64` targets, and `goreleaser release --clean`
builds every target in the config, archives them, computes checksums,
generates the grouped changelog, and publishes the GitHub Release — the
same single command validated locally in Task 4 (`goreleaser release
--snapshot --clean`), just without `--snapshot` so it actually publishes.

- [ ] **Step 3: Validate the YAML with actionlint**

```bash
go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/tag-release-goreleaser.yml
```

Expected: no output.

- [ ] **Step 4: Run zizmor again across all three workflows now that this one exists**

```bash
uvx zizmor@latest .github/workflows/
```

Expected: no medium+ severity findings.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/tag-release-goreleaser.yml
git commit -m "ci: add tag-release-goreleaser workflow"
```

- [ ] **Step 6: Verify end-to-end by pushing a real tag**

This is the only step that proves the release pipeline actually works on
GitHub's infrastructure (in particular, that cgo cross-compilation to
darwin/amd64 succeeds on a real `macos-latest` runner the same way it did
locally in Task 4) — it cannot be verified locally.
**Confirm with the user before doing this**, since pushing a tag is a
shared, hard-to-fully-reverse action (it creates a public GitHub Release).
Once confirmed:

```bash
git tag v0.3.3
git push origin main --tags
```

Then watch the Actions tab for the `tag-release-goreleaser` run, and check
the resulting GitHub Release has both `.tar.gz` archives, a
`checksums.txt`, and grouped release notes.

---

## Task 6: README note on unsigned macOS binaries

**Files:**
- Modify: `README.md` (Installation section, added earlier — add a subsection after the existing `go install`/build-from-source instructions)

**Interfaces:**
- Consumes: nothing.
- Produces: nothing.

- [ ] **Step 1: Read the current Installation section to find the exact insertion point**

```bash
grep -n "## Installation\|## Usage" README.md
```

- [ ] **Step 2: Add the Gatekeeper note**

Insert this new subsection immediately before the `## Usage` heading (i.e.
right after the existing "From source" build instructions):

```markdown
**From a downloaded release binary:**

Download the `.tar.gz` for your Mac's architecture from the
[Releases page](https://github.com/jonasbn/somafm-player/releases), then:

```sh
tar -xzf somafm-player_*_darwin_*.tar.gz
xattr -d com.apple.quarantine somafm-player
./somafm-player
```

The binary isn't signed or notarized yet, so macOS Gatekeeper quarantines
it on download — the `xattr` command above clears that flag. Without it,
double-clicking or running the binary will show an "cannot be opened
because the developer cannot be verified" dialog.
```

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: document unsigned macOS release binary Gatekeeper workaround"
```

---

## Self-review notes

- **Spec coverage:** Task 1 → CI goal (build/test/lint/govulncheck). Task 2
  → zizmor goal. Task 3 → Dependabot goal. Tasks 4–5 → release goal
  (GoReleaser config + split-build workflow). Task 6 → the unsigned-binary
  non-goal's required README caveat. All five numbered goals and the
  relevant non-goals in the spec are covered.
- **Placeholders:** The `<...-sha>` markers are intentional — they're
  filled in by a documented lookup command in Step 1 of each task that uses
  them, not left as undone thinking. Every other step has literal, complete
  content.
- **Type/name consistency:** Workflow file names match the spec's table
  exactly. `.goreleaser.yaml` build `id`s (`darwin-amd64`, `darwin-arm64`)
  are referenced identically in Task 5's `--id` flags. Binary name
  `somafm-player` matches the module name and existing `.gitignore` entry.
