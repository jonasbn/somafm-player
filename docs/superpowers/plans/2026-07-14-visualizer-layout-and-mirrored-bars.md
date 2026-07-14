# Visualizer Layout Move + Mirrored Bars Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the visualizer box beside the Now Playing box (falling back to today's stacked layout on narrow terminals) and grow its bar area from 1 row to a 4-row mirrored waveform, matching Now Playing's height.

**Architecture:** Two independent changes in `internal/ui`: (1) `renderVisualizerBox` gains a mirrored, 4-row rendering algorithm reusing the existing `barChar`/`gradientColor` helpers; (2) `View()` gains a width-driven choice between joining Now Playing and the visualizer horizontally (`lipgloss.JoinHorizontal`) or stacking them vertically as today.

**Tech Stack:** Go, Bubble Tea, Lipgloss (`github.com/charmbracelet/lipgloss`).

## Global Constraints

- Test isolation: any test touching the quit-key path or `config.Save`/`config.Load` must set `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` first. (Not applicable to this plan's tests — they don't touch config persistence — but noted since it's a project-wide rule.)
- Run `go mod tidy` after adding any new direct import. (Not expected in this plan — no new dependencies.)
- No TTY/audio hardware in sandboxed environments — actual terminal rendering must be manually verified via `go run .` by a human; agents verify via `go test ./...` and `go build ./...` only.
- IDE diagnostics shown after an edit can be stale — verify via `go build ./...`/`go test ./...`, not the diagnostics panel.

---

## File Structure

- Modify: `internal/ui/visualizer.go` — add `splitMirroredLevels` (per-column glyph split for the mirrored waveform) and rewrite `renderVisualizerBox` to render 4 rows instead of 1. Add `minSideBySideVisualizerWidth` constant.
- Modify: `internal/ui/visualizer_test.go` — add tests for `splitMirroredLevels` and the new multi-row `renderVisualizerBox` output.
- Modify: `internal/ui/view.go` — replace the `View()` section that unconditionally stacks Now Playing above the visualizer with `renderNowPlayingRow`, which picks side-by-side vs. stacked based on a new `sideBySideVisualizerWidth` method.
- Modify: `internal/ui/view_test.go` — add tests for `sideBySideVisualizerWidth` and for `View()`'s layout choice at wide vs. narrow terminal widths.

No new files, no new dependencies, no config/interface changes.

---

### Task 1: Mirrored 4-row visualizer rendering

**Files:**
- Modify: `internal/ui/visualizer.go`
- Test: `internal/ui/visualizer_test.go`

**Interfaces:**
- Consumes: existing `barChar(v float64) rune`, `gradientColor(v float64, t theme.Theme) lipgloss.Color`, `resampleBands(bands []float64, barCount int) []float64`, `displayBarCount(width int) int`, `borderStyle(t theme.Theme, focused bool) lipgloss.Style` (all already defined in `internal/ui/visualizer.go` and `internal/ui/view.go`).
- Produces: `splitMirroredLevels(v float64) (inner rune, outer rune, innerFilled bool, outerFilled bool)` — used by Task 2 only indirectly (Task 2 doesn't call it directly, but relies on `renderVisualizerBox` now producing 4 lines). `renderVisualizerBox(t theme.Theme, width int) string` keeps its existing signature but now returns a 6-line string (border + 4 content rows + border) instead of 3 lines.

- [ ] **Step 1: Write the failing tests**

Add to `internal/ui/visualizer_test.go` (append at the end of the file):

```go
func TestSplitMirroredLevels_ZeroIsBlankBothRows(t *testing.T) {
	_, _, innerFilled, outerFilled := splitMirroredLevels(0)
	if innerFilled || outerFilled {
		t.Fatalf("splitMirroredLevels(0) innerFilled=%v outerFilled=%v, want both false", innerFilled, outerFilled)
	}
}

func TestSplitMirroredLevels_LowValueFillsInnerRowOnly(t *testing.T) {
	inner, _, innerFilled, outerFilled := splitMirroredLevels(0.3)
	if !innerFilled {
		t.Fatalf("splitMirroredLevels(0.3) innerFilled = false, want true")
	}
	if outerFilled {
		t.Fatalf("splitMirroredLevels(0.3) outerFilled = true, want false (v=0.3 is in the inner-only range [0,0.5])")
	}
	if want := barChar(0.6); inner != want {
		t.Fatalf("splitMirroredLevels(0.3) inner = %q, want %q (barChar(0.3*2))", inner, want)
	}
}

func TestSplitMirroredLevels_HighValueMaxesInnerAndPartiallyFillsOuter(t *testing.T) {
	inner, outer, innerFilled, outerFilled := splitMirroredLevels(0.8)
	if !innerFilled || !outerFilled {
		t.Fatalf("splitMirroredLevels(0.8) innerFilled=%v outerFilled=%v, want both true", innerFilled, outerFilled)
	}
	if want := barChar(1); inner != want {
		t.Fatalf("splitMirroredLevels(0.8) inner = %q, want %q (maxed out)", inner, want)
	}
	if want := barChar(0.6); outer != want {
		t.Fatalf("splitMirroredLevels(0.8) outer = %q, want %q (barChar((0.8-0.5)*2))", outer, want)
	}
}

func TestSplitMirroredLevels_ClampsOutOfRangeInput(t *testing.T) {
	innerLo, outerLo, innerFilledLo, outerFilledLo := splitMirroredLevels(-1)
	innerZero, outerZero, innerFilledZero, outerFilledZero := splitMirroredLevels(0)
	if innerLo != innerZero || outerLo != outerZero || innerFilledLo != innerFilledZero || outerFilledLo != outerFilledZero {
		t.Fatalf("splitMirroredLevels(-1) = (%q,%q,%v,%v), want same as splitMirroredLevels(0) = (%q,%q,%v,%v)",
			innerLo, outerLo, innerFilledLo, outerFilledLo, innerZero, outerZero, innerFilledZero, outerFilledZero)
	}

	innerHi, outerHi, innerFilledHi, outerFilledHi := splitMirroredLevels(2)
	innerOne, outerOne, innerFilledOne, outerFilledOne := splitMirroredLevels(1)
	if innerHi != innerOne || outerHi != outerOne || innerFilledHi != innerFilledOne || outerFilledHi != outerFilledOne {
		t.Fatalf("splitMirroredLevels(2) = (%q,%q,%v,%v), want same as splitMirroredLevels(1) = (%q,%q,%v,%v)",
			innerHi, outerHi, innerFilledHi, outerFilledHi, innerOne, outerOne, innerFilledOne, outerFilledOne)
	}
}

func TestRenderVisualizerBox_RendersFourMirroredContentRows(t *testing.T) {
	m := newTestModel()
	m.bands = []float64{0.9, 0.1, 0.5, 0.9, 0.1, 0.5, 0.9, 0.1}
	out := m.renderVisualizerBox(theme.Get("Nord"), 8)

	lines := strings.Split(out, "\n")
	if len(lines) != 6 { // top border + 4 content rows + bottom border
		t.Fatalf("renderVisualizerBox() produced %d lines, want 6 (border+4 content rows+border):\n%s", len(lines), out)
	}
}

func TestRenderVisualizerBox_TopAndBottomRowsMirrorEachOther(t *testing.T) {
	m := newTestModel()
	m.bands = []float64{0.9, 0.1, 0.5, 0.9, 0.1, 0.5, 0.9, 0.1}
	out := m.renderVisualizerBox(theme.Get("Nord"), 8)

	lines := strings.Split(out, "\n")
	// lines[0]=top border, lines[1]=outer-above, lines[2]=inner-above,
	// lines[3]=inner-below, lines[4]=outer-below, lines[5]=bottom border.
	if lines[1] != lines[4] {
		t.Fatalf("outer rows do not mirror:\n top=%q\n bottom=%q", lines[1], lines[4])
	}
	if lines[2] != lines[3] {
		t.Fatalf("inner rows do not mirror:\n top=%q\n bottom=%q", lines[2], lines[3])
	}
}
```

This requires adding `"strings"` to the test file's imports — check first: `internal/ui/visualizer_test.go` currently imports `"math"`, `"testing"`, `go-colorful`, and `theme`. Add `"strings"` alongside `"math"`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ui/... -run 'TestSplitMirroredLevels|TestRenderVisualizerBox' -v`
Expected: FAIL — `undefined: splitMirroredLevels`, and the two `TestRenderVisualizerBox_*` tests fail on the line-count/mirroring assertions (current `renderVisualizerBox` only produces 3 lines).

- [ ] **Step 3: Implement `splitMirroredLevels` and rewrite `renderVisualizerBox`**

In `internal/ui/visualizer.go`, add this constant next to `minDisplayBars`/`maxDisplayBars`:

```go
// minSideBySideVisualizerWidth is the content-column floor below which the
// visualizer stops rendering beside Now Playing and falls back to the
// stacked, full-width layout (see view.go's sideBySideVisualizerWidth) —
// set 2 above minDisplayBars so the side-by-side layout gives up before
// bars would get unreadably dense anyway.
const minSideBySideVisualizerWidth = minDisplayBars + 2
```

Add this function after `gradientColor`:

```go
// splitMirroredLevels maps a 0.0-1.0 band value onto the mirrored waveform
// display's two rows per side: the inner row (touching the implicit center
// line) fills first for v in [0, 0.5] via barChar(v*2); once maxed, the
// outer row fills for the remainder, v in [0.5, 1.0], via
// barChar((v-0.5)*2). innerFilled/outerFilled report whether each row
// should render a glyph at all — v==0 renders both rows blank so silence
// reads as empty space rather than the flat baseline glyph barChar(0)
// would otherwise produce.
func splitMirroredLevels(v float64) (inner rune, outer rune, innerFilled bool, outerFilled bool) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	if v == 0 {
		return 0, 0, false, false
	}
	inner = barChar(v * 2)
	if v <= 0.5 {
		return inner, 0, true, false
	}
	outer = barChar((v - 0.5) * 2)
	return inner, outer, true, true
}
```

Replace the existing `renderVisualizerBox` with:

```go
func (m Model) renderVisualizerBox(t theme.Theme, width int) string {
	bars := resampleBands(m.bands, displayBarCount(width))

	var innerRow, outerRow strings.Builder
	for _, v := range bars {
		style := lipgloss.NewStyle().Foreground(gradientColor(v, t))
		inner, outer, innerFilled, outerFilled := splitMirroredLevels(v)
		if innerFilled {
			innerRow.WriteString(style.Render(string(inner)))
		} else {
			innerRow.WriteString(" ")
		}
		if outerFilled {
			outerRow.WriteString(style.Render(string(outer)))
		} else {
			outerRow.WriteString(" ")
		}
	}

	inner := innerRow.String()
	outer := outerRow.String()
	body := strings.Join([]string{outer, inner, inner, outer}, "\n")
	return borderStyle(t, false).Width(width).Render(body)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/ui/... -run 'TestSplitMirroredLevels|TestRenderVisualizerBox|TestBarChar|TestResampleBands|TestDisplayBarCount|TestGradientColor' -v`
Expected: PASS for all.

- [ ] **Step 5: Run the full test suite and build**

Run: `go build ./... && go test ./...`
Expected: build succeeds; all tests pass, including the pre-existing `TestView_ShowsVisualizerBoxWhenEnabled` / `TestView_HidesVisualizerBoxByDefault` in `view_test.go` (unaffected — they only check for the presence/absence of bar characters, not row count).

- [ ] **Step 6: Commit**

```bash
git add internal/ui/visualizer.go internal/ui/visualizer_test.go
git commit -m "feat(ui): render visualizer as a 4-row mirrored waveform"
```

---

### Task 2: Side-by-side layout with Now Playing

**Files:**
- Modify: `internal/ui/view.go`
- Test: `internal/ui/view_test.go`

**Interfaces:**
- Consumes: `m.renderNowPlaying(t theme.Theme) string` (unchanged, existing), `m.renderVisualizerBox(t theme.Theme, width int) string` (unchanged signature, now 4 rows tall per Task 1), `m.fullBoxWidth() int` (unchanged, existing), `minSideBySideVisualizerWidth` (defined in Task 1's `visualizer.go`), `defaultWidth` and `decorationPerBox` constants (existing).
- Produces: `(m Model) sideBySideVisualizerWidth(nowPlayingWidth int) (width int, ok bool)` and `(m Model) renderNowPlayingRow(t theme.Theme) string` — both new, used only within `view.go`'s `View()`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/ui/view_test.go`:

```go
func TestSideBySideVisualizerWidth_OkWhenLeftoverClearsFloor(t *testing.T) {
	m := newTestModel()
	m.width = 100

	width, ok := m.sideBySideVisualizerWidth(50)
	// 100 - 50 - 4 (decorationPerBox) = 46
	if !ok || width != 46 {
		t.Fatalf("sideBySideVisualizerWidth(50) = (%d, %v), want (46, true)", width, ok)
	}
}

func TestSideBySideVisualizerWidth_FallsBackBelowFloor(t *testing.T) {
	m := newTestModel()
	m.width = 60

	width, ok := m.sideBySideVisualizerWidth(50)
	// 60 - 50 - 4 = 6, below minSideBySideVisualizerWidth (10)
	if ok {
		t.Fatalf("sideBySideVisualizerWidth(50) with m.width=60 = (%d, %v), want ok=false", width, ok)
	}
}

func TestSideBySideVisualizerWidth_FallsBackToDefaultWidthWhenZero(t *testing.T) {
	m := newTestModel()
	m.width = 0

	width, ok := m.sideBySideVisualizerWidth(50)
	// defaultWidth (80) - 50 - 4 = 26
	if !ok || width != 26 {
		t.Fatalf("sideBySideVisualizerWidth(50) = (%d, %v), want (26, true) when m.width is unset", width, ok)
	}
}

func TestView_PlacesVisualizerBesideNowPlayingWhenWideEnough(t *testing.T) {
	m := newTestModel()
	m.cfg.VisualizerEnabled = true
	m.width = 200
	out := m.View()

	found := false
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "♪") && strings.ContainsAny(line, "▁▂▃▄▅▆▇█") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("View() at width=200 should have a line containing both Now Playing content and visualizer bars (side-by-side layout):\n%s", out)
	}
}

func TestView_StacksVisualizerBelowNowPlayingWhenNarrow(t *testing.T) {
	m := newTestModel()
	m.cfg.VisualizerEnabled = true
	m.width = 20
	out := m.View()

	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "♪") && strings.ContainsAny(line, "▁▂▃▄▅▆▇█") {
			t.Fatalf("View() at width=20 should stack (not place bars on the same line as Now Playing):\n%s", out)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ui/... -run 'TestSideBySideVisualizerWidth|TestView_PlacesVisualizerBesideNowPlaying|TestView_StacksVisualizerBelowNowPlaying' -v`
Expected: FAIL — `undefined: m.sideBySideVisualizerWidth` (compile error covers all four new tests).

- [ ] **Step 3: Implement `sideBySideVisualizerWidth` and `renderNowPlayingRow`, wire into `View()`**

In `internal/ui/view.go`, replace:

```go
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	t := theme.Get(m.cfg.Theme)

	width := m.boxWidth()
	lists := lipgloss.JoinHorizontal(lipgloss.Top, m.renderChannelsBox(t, width), m.renderTunesBox(t, width))

	sections := []string{m.renderNowPlaying(t)}
	if m.cfg.VisualizerEnabled {
		sections = append(sections, m.renderVisualizerBox(t, m.fullBoxWidth()))
	}
	sections = append(sections, lists)

	footer := fmt.Sprintf("[Theme: %s]  tab focus · j/k move · enter play · b bookmark · a all/bookmarked · H/s tunes · +/- vol · m mute · t theme · v visualizer · r retry channels · q quit", t.Name)
	if m.errMsg != "" {
		footer = "Error: " + m.errMsg + "\n" + footer
	}
	sections = append(sections, footer)

	return strings.Join(sections, "\n")
}
```

with:

```go
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	t := theme.Get(m.cfg.Theme)

	width := m.boxWidth()
	lists := lipgloss.JoinHorizontal(lipgloss.Top, m.renderChannelsBox(t, width), m.renderTunesBox(t, width))

	sections := []string{m.renderNowPlayingRow(t), lists}

	footer := fmt.Sprintf("[Theme: %s]  tab focus · j/k move · enter play · b bookmark · a all/bookmarked · H/s tunes · +/- vol · m mute · t theme · v visualizer · r retry channels · q quit", t.Name)
	if m.errMsg != "" {
		footer = "Error: " + m.errMsg + "\n" + footer
	}
	sections = append(sections, footer)

	return strings.Join(sections, "\n")
}

// renderNowPlayingRow renders Now Playing alone, or joined with the
// visualizer when enabled. It picks side-by-side (when there's enough
// leftover terminal width after Now Playing's content-sized box) or
// stacked, full-width — today's layout, kept as a fallback so the
// visualizer never gets squeezed down to an unreadable sliver on narrow
// terminals.
func (m Model) renderNowPlayingRow(t theme.Theme) string {
	nowPlaying := m.renderNowPlaying(t)
	if !m.cfg.VisualizerEnabled {
		return nowPlaying
	}

	if leftover, ok := m.sideBySideVisualizerWidth(lipgloss.Width(nowPlaying)); ok {
		return lipgloss.JoinHorizontal(lipgloss.Top, nowPlaying, m.renderVisualizerBox(t, leftover))
	}
	return strings.Join([]string{nowPlaying, m.renderVisualizerBox(t, m.fullBoxWidth())}, "\n")
}

// sideBySideVisualizerWidth returns the content width available for the
// visualizer box if placed beside an already-rendered Now Playing box of
// the given width, and whether that leftover width clears the floor
// needed to still look like a bar chart (minSideBySideVisualizerWidth,
// defined in visualizer.go) rather than a sliver. Below the floor,
// callers should fall back to the stacked layout.
func (m Model) sideBySideVisualizerWidth(nowPlayingWidth int) (width int, ok bool) {
	w := m.width
	if w <= 0 {
		w = defaultWidth
	}
	leftover := w - nowPlayingWidth - decorationPerBox
	return leftover, leftover >= minSideBySideVisualizerWidth
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/ui/... -run 'TestSideBySideVisualizerWidth|TestView_' -v`
Expected: PASS for all, including the pre-existing `TestView_HidesVisualizerBoxByDefault`, `TestView_ShowsVisualizerBoxWhenEnabled`, `TestView_FooterMentionsVisualizerKey`, `TestView_DoesNotPanicWhenQuitting`, `TestView_RendersBothBoxesAndFooter`.

- [ ] **Step 5: Run the full test suite and build**

Run: `go build ./... && go test ./...`
Expected: build succeeds, all tests pass.

- [ ] **Step 6: Manual verification (human, not agent)**

Per CLAUDE.md, actual terminal rendering can't be verified in a sandboxed agent environment. A human should run `go run .`, press `v` to enable the visualizer, and confirm: (a) at a normal terminal width, Now Playing and the visualizer sit side by side with aligned borders and matching height; (b) resizing the terminal narrower causes the visualizer to drop below Now Playing (stacked) rather than squeezing to a sliver; (c) the bars mirror top-to-bottom and animate smoothly during playback.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/view.go internal/ui/view_test.go
git commit -m "feat(ui): place visualizer beside Now Playing with a stacked fallback"
```
