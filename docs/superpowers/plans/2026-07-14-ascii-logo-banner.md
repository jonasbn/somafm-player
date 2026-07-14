# ASCII Logo Banner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render an ASCII-art banner above the Now Playing row — a default "somafm" logo normally, switching to a Drone Zone or Deep Space One logo when that channel is currently playing.

**Architecture:** A new, dependency-free `internal/logo` package holds the three ASCII art blocks and a `For(channelTitle)` lookup. `internal/ui` gets a small `renderLogo` that picks the lookup result, colors it via the active theme, and falls back to a one-line "SomaFM" label when the terminal is too narrow for the art. `View()` prepends the rendered banner to its existing sections.

**Tech Stack:** Go 1.26, `github.com/charmbracelet/lipgloss` (already a dependency — no new imports needed).

## Global Constraints

- The banner is always rendered — no toggle, no keybinding, no config field (spec's `docs/superpowers/specs/2026-07-14-ascii-logo-banner-design.md` Non-goals).
- Channel matching is exact-string on `m.nowPlaying.channel`: `"Drone Zone"` and `"Drone Zone 2"` → Drone Zone art; `"Deep Space One"` → Deep Space One art; anything else (including `""`, nothing playing) → default "somafm" art.
- Default art is colored with the active theme's `Hot` color; channel-specific art is colored with the active theme's `Accent` color.
- No border/padding on the banner — plain colored text, unlike Now Playing/Channels/Tunes.
- Below-width fallback is always the literal text `"SomaFM"` on a single line, never the channel name or a clipped/wrapped render of the art.
- ASCII art content must exactly match `docs/TODO.md` lines 6–12 (default), 14–19 (Drone Zone), 21–25 (Deep Space One) — verified byte-for-byte against the source file, not retyped from memory.

---

### Task 1: `internal/logo` package

**Files:**
- Create: `internal/logo/logo.go`
- Test: `internal/logo/logo_test.go`

**Interfaces:**
- Produces: `logo.For(channelTitle string) (lines []string, isDefault bool)` — `isDefault` is `true` when `channelTitle` didn't match a known channel and the default art was returned.
- Produces: `logo.Width(lines []string) int` — the length of the widest line in `lines` (all art is pure ASCII, so `len()` in bytes equals display width).

- [ ] **Step 1: Write the failing test**

Create `internal/logo/logo_test.go`:

```go
package logo

import "testing"

func TestFor_DroneZoneReturnsDroneZoneArt(t *testing.T) {
	lines, isDefault := For("Drone Zone")
	if isDefault {
		t.Fatal("For(\"Drone Zone\") isDefault = true, want false")
	}
	if len(lines) != len(droneZoneArt) || lines[0] != droneZoneArt[0] {
		t.Fatalf("For(\"Drone Zone\") = %v, want droneZoneArt", lines)
	}
}

func TestFor_DroneZone2ReturnsSameArtAsDroneZone(t *testing.T) {
	lines, isDefault := For("Drone Zone 2")
	if isDefault {
		t.Fatal("For(\"Drone Zone 2\") isDefault = true, want false")
	}
	if len(lines) != len(droneZoneArt) || lines[0] != droneZoneArt[0] {
		t.Fatalf("For(\"Drone Zone 2\") = %v, want droneZoneArt", lines)
	}
}

func TestFor_DeepSpaceOneReturnsDeepSpaceOneArt(t *testing.T) {
	lines, isDefault := For("Deep Space One")
	if isDefault {
		t.Fatal("For(\"Deep Space One\") isDefault = true, want false")
	}
	if len(lines) != len(deepSpaceOneArt) || lines[0] != deepSpaceOneArt[0] {
		t.Fatalf("For(\"Deep Space One\") = %v, want deepSpaceOneArt", lines)
	}
}

func TestFor_UnmatchedTitleReturnsDefaultArt(t *testing.T) {
	for _, title := range []string{"", "Groove Salad", "Drone Zone 3", "not a channel"} {
		lines, isDefault := For(title)
		if !isDefault {
			t.Errorf("For(%q) isDefault = false, want true", title)
		}
		if len(lines) != len(defaultArt) || lines[0] != defaultArt[0] {
			t.Errorf("For(%q) = %v, want defaultArt", title, lines)
		}
	}
}

func TestWidth_ReturnsWidestLineLength(t *testing.T) {
	lines := []string{"ab", "abcde", "abc"}
	if got := Width(lines); got != 5 {
		t.Fatalf("Width(lines) = %d, want 5", got)
	}
}

func TestWidth_EmptySliceReturnsZero(t *testing.T) {
	if got := Width(nil); got != 0 {
		t.Fatalf("Width(nil) = %d, want 0", got)
	}
}

func TestArt_MeasuredWidthsMatchTODOSource(t *testing.T) {
	// docs/TODO.md's ASCII art was measured (via `awk '{print length}'`) at
	// these exact widths when the spec was written. This guards against a
	// future edit to the art silently changing dimensions the rendering
	// logic (internal/ui/logo.go's width-fallback check) depends on.
	if got := Width(defaultArt); got != 44 {
		t.Errorf("Width(defaultArt) = %d, want 44", got)
	}
	if got := Width(droneZoneArt); got != 56 {
		t.Errorf("Width(droneZoneArt) = %d, want 56", got)
	}
	if got := Width(deepSpaceOneArt); got != 61 {
		t.Errorf("Width(deepSpaceOneArt) = %d, want 61", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/logo/... -v`
Expected: FAIL — `internal/logo` package doesn't exist yet (`no Go files in ...` or undefined `For`/`Width`/`droneZoneArt`/etc.)

- [ ] **Step 3: Write the implementation**

Create `internal/logo/logo.go`. The art content below is copied verbatim (byte-for-byte, including trailing spaces) from `docs/TODO.md` lines 6–12, 14–19, and 21–25 — do not retype it by hand; if you need to regenerate it, read those exact line ranges from `docs/TODO.md` and escape backslashes for Go double-quoted strings (the font uses literal backslash and backtick characters, which is why this is `[]string` of interpreted strings rather than a raw `` ` ``-delimited string):

```go
// Package logo provides the ASCII-art banner shown above the player,
// selecting a channel-specific variant when one is currently playing.
package logo

// defaultArt is the "somafm" banner (patorjk.com "Big" font), shown when
// no channel-specific art matches.
var defaultArt = []string{
	"                               __           ",
	"                              / _|          ",
	"  ___  ___  _ __ ___   __ _  | |_ _ __ ___  ",
	" / __|/ _ \\| '_ ` _ \\ / _` | |  _| '_ ` _ \\ ",
	" \\__ \\ (_) | | | | | | (_| | | | | | | | | |",
	" |___/\\___/|_| |_| |_|\\__,_| |_| |_| |_| |_|",
	"                                            ",
}

// droneZoneArt is shown for the "Drone Zone" and "Drone Zone 2" channels.
var droneZoneArt = []string{
	"  ____  ____   ___  _   _ _____   ________  _   _ _____ ",
	" |  _ \\|  _ \\ / _ \\| \\ | | ____| |__  / _ \\| \\ | | ____|",
	" | | | | |_) | | | |  \\| |  _|     / / | | |  \\| |  _|  ",
	" | |_| |  _ <| |_| | |\\  | |___   / /| |_| | |\\  | |___ ",
	" |____/|_| \\_\\\\___/|_| \\_|_____| /____\\___/|_| \\_|_____|",
	"                                                        ",
}

// deepSpaceOneArt is shown for the "Deep Space One" channel.
var deepSpaceOneArt = []string{
	" ____                 _____                    _____         ",
	"|    \\ ___ ___ ___   |   __|___ ___ ___ ___   |     |___ ___ ",
	"|  |  | -_| -_| . |  |__   | . | .'|  _| -_|  |  |  |   | -_|",
	"|____/|___|___|  _|  |_____|  _|__,|___|___|  |_____|_|_|___|",
	"              |_|          |_|",
}

var byChannel = map[string][]string{
	"Drone Zone":     droneZoneArt,
	"Drone Zone 2":   droneZoneArt,
	"Deep Space One": deepSpaceOneArt,
}

// For returns the ASCII art lines for channelTitle, falling back to the
// default "somafm" art for any unmatched title (including ""). isDefault
// reports whether the fallback was used.
func For(channelTitle string) (lines []string, isDefault bool) {
	if art, ok := byChannel[channelTitle]; ok {
		return art, false
	}
	return defaultArt, true
}

// Width returns the length of the widest line in lines. All art is pure
// ASCII, so byte length equals display width.
func Width(lines []string) int {
	max := 0
	for _, l := range lines {
		if len(l) > max {
			max = len(l)
		}
	}
	return max
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/logo/... -v`
Expected: PASS (all 7 tests)

- [ ] **Step 5: Check formatting**

Run: `gofmt -l internal/logo`
Expected: no output (empty = clean)

- [ ] **Step 6: Commit**

```bash
git add internal/logo/logo.go internal/logo/logo_test.go
git commit -m "$(cat <<'EOF'
feat(logo): add channel-aware ASCII logo lookup

Adds internal/logo with the default "somafm" banner plus Drone Zone
and Deep Space One variants from docs/TODO.md, selected by channel
title via logo.For().
EOF
)"
```

---

### Task 2: Render the banner in the UI

**Files:**
- Create: `internal/ui/logo.go`
- Create: `internal/ui/logo_test.go`
- Modify: `internal/ui/view.go:167-176` (the `View()` function's `sections` slice)
- Test: `internal/ui/view_test.go` (add one integration test)

**Interfaces:**
- Consumes: `logo.For(channelTitle string) (lines []string, isDefault bool)` and `logo.Width(lines []string) int` from Task 1.
- Consumes: `Model.nowPlaying.channel string` (`internal/ui/model.go`), `Model.width int`, `defaultWidth` constant (`internal/ui/model.go:38`), `theme.Theme{Hot, Accent lipgloss.Color}` (`internal/theme/theme.go`).
- Produces: `(m Model) renderLogo(t theme.Theme) string` — used by `View()`.
- Produces: `logoColor(t theme.Theme, isDefault bool) lipgloss.Color` — a standalone, directly-testable helper (kept separate from `renderLogo` so tests can assert the color decision without needing to parse ANSI-styled output, matching how `gradientColor` in `internal/ui/visualizer.go` is tested).
- Produces: `logoFallbackText` constant (`"SomaFM"`).

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/logo_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"github.com/jonasbn/somafm-player/internal/theme"
)

func TestLogoColor_DefaultUsesThemeHot(t *testing.T) {
	th := theme.Get("Dracula")
	if got := logoColor(th, true); got != th.Hot {
		t.Fatalf("logoColor(isDefault=true) = %v, want theme Hot %v", got, th.Hot)
	}
}

func TestLogoColor_ChannelSpecificUsesThemeAccent(t *testing.T) {
	th := theme.Get("Dracula")
	if got := logoColor(th, false); got != th.Accent {
		t.Fatalf("logoColor(isDefault=false) = %v, want theme Accent %v", got, th.Accent)
	}
}

func TestRenderLogo_WideEnoughShowsFullArtForDefaultChannel(t *testing.T) {
	m := newTestModel()
	m.width = 100 // wider than every logo (widest is 61 cols)
	th := theme.Get(m.cfg.Theme)

	got := m.renderLogo(th)

	wantLineCount := 7 // len(defaultArt); nowPlaying.channel is "" on a fresh model
	if gotLineCount := len(strings.Split(got, "\n")); gotLineCount != wantLineCount {
		t.Fatalf("renderLogo() produced %d lines, want %d (full default art):\n%s", gotLineCount, wantLineCount, got)
	}
	if strings.Contains(got, logoFallbackText) {
		t.Fatalf("renderLogo() at width=100 = %q, should not contain fallback text", got)
	}
}

func TestRenderLogo_WideEnoughShowsChannelArtWhenPlayingDroneZone(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.nowPlaying.channel = "Drone Zone"
	th := theme.Get(m.cfg.Theme)

	got := m.renderLogo(th)

	wantLineCount := 6 // len(droneZoneArt)
	if gotLineCount := len(strings.Split(got, "\n")); gotLineCount != wantLineCount {
		t.Fatalf("renderLogo() produced %d lines, want %d (Drone Zone art):\n%s", gotLineCount, wantLineCount, got)
	}
}

func TestRenderLogo_NarrowWidthShowsSingleLineFallback(t *testing.T) {
	m := newTestModel()
	m.width = 10 // narrower than every logo (narrowest is 44 cols)
	th := theme.Get(m.cfg.Theme)

	got := m.renderLogo(th)

	if !strings.Contains(got, logoFallbackText) {
		t.Fatalf("renderLogo() at width=10 = %q, want it to contain fallback text %q", got, logoFallbackText)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("renderLogo() at width=10 = %q, want a single line (no newlines)", got)
	}
}

func TestRenderLogo_ZeroWidthFallsBackToDefaultWidthNotFallback(t *testing.T) {
	m := newTestModel()
	m.width = 0 // unset: renderLogo must treat this as defaultWidth (80), same as boxWidth/fullBoxWidth do

	got := m.renderLogo(theme.Get(m.cfg.Theme))

	// defaultWidth (80) clears every logo's width (widest is 61), so the
	// full 7-line default art should render, not the 1-line fallback.
	if gotLineCount := len(strings.Split(got, "\n")); gotLineCount != 7 {
		t.Fatalf("renderLogo() at width=0 produced %d lines, want 7 (defaultWidth should clear every logo's width):\n%s", gotLineCount, got)
	}
}
```

- [ ] **Step 2: Add the `View()` integration test**

Append to `internal/ui/view_test.go`:

```go
func TestView_StartsWithRenderedLogo(t *testing.T) {
	m := newTestModel()
	th := theme.Get(m.cfg.Theme)
	wantLogo := m.renderLogo(th)

	out := m.View()

	if !strings.HasPrefix(out, wantLogo) {
		t.Fatalf("View() does not start with renderLogo() output.\nView():\n%s\nrenderLogo():\n%s", out, wantLogo)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/ui/... -v -run 'TestLogoColor|TestRenderLogo|TestView_StartsWithRenderedLogo'`
Expected: FAIL to compile — `undefined: logoColor`, `undefined: logoFallbackText`, `m.renderLogo undefined`

- [ ] **Step 4: Write `internal/ui/logo.go`**

```go
package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jonasbn/somafm-player/internal/logo"
	"github.com/jonasbn/somafm-player/internal/theme"
)

// logoFallbackText replaces the full ASCII art when the terminal is too
// narrow to fit it without wrapping. Always this literal text, never the
// channel name — the Now Playing box already shows the channel.
const logoFallbackText = "SomaFM"

// logoColor picks the banner's color: the default "somafm" art reads as
// red/warm via each theme's Hot color (already a red-family accent in
// every theme), while channel-specific art uses the theme's normal
// Accent color.
func logoColor(t theme.Theme, isDefault bool) lipgloss.Color {
	if isDefault {
		return t.Hot
	}
	return t.Accent
}

// renderLogo renders the ASCII banner for the currently playing channel
// (or the default "somafm" art when nothing matches), falling back to a
// single-line label below the art's required width. Unlike the other
// panels, it has no border or padding.
func (m Model) renderLogo(t theme.Theme) string {
	lines, isDefault := logo.For(m.nowPlaying.channel)
	style := lipgloss.NewStyle().Foreground(logoColor(t, isDefault))

	w := m.width
	if w <= 0 {
		w = defaultWidth
	}
	if w < logo.Width(lines) {
		return style.Render(logoFallbackText)
	}
	return style.Render(strings.Join(lines, "\n"))
}
```

- [ ] **Step 5: Wire the banner into `View()`**

In `internal/ui/view.go`, change:

```go
	sections := []string{m.renderNowPlayingRow(t), lists}
```

to:

```go
	sections := []string{m.renderLogo(t), m.renderNowPlayingRow(t), lists}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/ui/... -v -run 'TestLogoColor|TestRenderLogo|TestView_StartsWithRenderedLogo'`
Expected: PASS (6 tests)

- [ ] **Step 7: Run the full test suite and build**

Run: `go build ./... && go test ./...`
Expected: both succeed, no failures (this also catches any existing test in `internal/ui` that asserted an exact `View()` line count/layout that the new banner row would now break — if one fails, update its expected line count rather than removing the assertion)

- [ ] **Step 8: Check formatting**

Run: `gofmt -l internal/ui`
Expected: no output (empty = clean)

- [ ] **Step 9: Commit**

```bash
git add internal/ui/logo.go internal/ui/logo_test.go internal/ui/view.go internal/ui/view_test.go
git commit -m "$(cat <<'EOF'
feat(ui): render ASCII logo banner above Now Playing

Adds renderLogo, using internal/logo to pick the default "somafm"
art or a channel-specific variant (Drone Zone, Deep Space One) based
on the currently playing channel, colored via the active theme
(Hot for default, Accent for channel-specific) and falling back to a
single-line "SomaFM" label on terminals too narrow for the art.
EOF
)"
```
