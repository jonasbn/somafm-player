package ui

import (
	"math"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"

	"github.com/jonasbn/somafm-player/internal/theme"
)

func TestBarChar_MapsLevelsAcrossRange(t *testing.T) {
	if got := barChar(0); got != barLevels[0] {
		t.Errorf("barChar(0) = %q, want %q", got, barLevels[0])
	}
	if got := barChar(1); got != barLevels[len(barLevels)-1] {
		t.Errorf("barChar(1) = %q, want %q", got, barLevels[len(barLevels)-1])
	}
}

func TestResampleBands_AveragesContiguousGroups(t *testing.T) {
	bands := []float64{0, 0, 1, 1} // 4 input bands -> 2 output bars
	got := resampleBands(bands, 2)
	want := []float64{0, 1}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestResampleBands_NilBandsProducesZeroFilledSlice(t *testing.T) {
	got := resampleBands(nil, 8)
	if len(got) != 8 {
		t.Fatalf("len = %d, want 8", len(got))
	}
	for i, v := range got {
		if v != 0 {
			t.Errorf("got[%d] = %v, want 0", i, v)
		}
	}
}

func TestResampleBands_NegativeBarCountDoesNotPanicReturnsEmpty(t *testing.T) {
	got := resampleBands([]float64{0.1, 0.2, 0.3}, -1)
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0 for negative barCount", len(got))
	}
}

func TestDisplayBarCount_ClampsToBounds(t *testing.T) {
	if got := displayBarCount(3); got != minDisplayBars {
		t.Errorf("displayBarCount(3) = %d, want %d", got, minDisplayBars)
	}
	if got := displayBarCount(100); got != maxDisplayBars {
		t.Errorf("displayBarCount(100) = %d, want %d", got, maxDisplayBars)
	}
	if got := displayBarCount(20); got != 20 {
		t.Errorf("displayBarCount(20) = %d, want 20", got)
	}
}

func TestGradientColor_EndpointsMatchThemeStops(t *testing.T) {
	th := theme.Get("Nord")
	cases := []struct {
		v    float64
		want string
	}{
		{0, string(th.Muted)},
		{0.5, string(th.Accent)},
		{1, string(th.Hot)},
	}
	for _, c := range cases {
		got := gradientColor(c.v, th)
		gotC, _ := colorful.Hex(string(got))
		wantC, _ := colorful.Hex(c.want)
		if dist := gotC.DistanceLab(wantC); dist > 0.01 {
			t.Errorf("gradientColor(%v) = %v, want close to %v (Lab distance %v)", c.v, got, c.want, dist)
		}
	}
}

func TestGradientColor_ClampsOutOfRangeInput(t *testing.T) {
	th := theme.Get("Nord")
	if got, zero := gradientColor(-1, th), gradientColor(0, th); got != zero {
		t.Errorf("gradientColor(-1) = %v, want same as gradientColor(0) = %v", got, zero)
	}
	if got, one := gradientColor(2, th), gradientColor(1, th); got != one {
		t.Errorf("gradientColor(2) = %v, want same as gradientColor(1) = %v", got, one)
	}
}

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
	if want := 0.6; math.Abs(inner-want) > 1e-9 {
		t.Fatalf("splitMirroredLevels(0.3) inner fraction = %v, want %v (0.3*2)", inner, want)
	}
}

func TestSplitMirroredLevels_HighValueMaxesInnerAndPartiallyFillsOuter(t *testing.T) {
	inner, outer, innerFilled, outerFilled := splitMirroredLevels(0.8)
	if !innerFilled || !outerFilled {
		t.Fatalf("splitMirroredLevels(0.8) innerFilled=%v outerFilled=%v, want both true", innerFilled, outerFilled)
	}
	if want := 1.0; math.Abs(inner-want) > 1e-9 {
		t.Fatalf("splitMirroredLevels(0.8) inner fraction = %v, want %v (maxed out)", inner, want)
	}
	if want := 0.6; math.Abs(outer-want) > 1e-9 {
		t.Fatalf("splitMirroredLevels(0.8) outer fraction = %v, want %v ((0.8-0.5)*2)", outer, want)
	}
}

func TestSplitMirroredLevels_ClampsOutOfRangeInput(t *testing.T) {
	innerLo, outerLo, innerFilledLo, outerFilledLo := splitMirroredLevels(-1)
	innerZero, outerZero, innerFilledZero, outerFilledZero := splitMirroredLevels(0)
	if innerLo != innerZero || outerLo != outerZero || innerFilledLo != innerFilledZero || outerFilledLo != outerFilledZero {
		t.Fatalf("splitMirroredLevels(-1) = (%v,%v,%v,%v), want same as splitMirroredLevels(0) = (%v,%v,%v,%v)",
			innerLo, outerLo, innerFilledLo, outerFilledLo, innerZero, outerZero, innerFilledZero, outerFilledZero)
	}

	innerHi, outerHi, innerFilledHi, outerFilledHi := splitMirroredLevels(2)
	innerOne, outerOne, innerFilledOne, outerFilledOne := splitMirroredLevels(1)
	if innerHi != innerOne || outerHi != outerOne || innerFilledHi != innerFilledOne || outerFilledHi != outerFilledOne {
		t.Fatalf("splitMirroredLevels(2) = (%v,%v,%v,%v), want same as splitMirroredLevels(1) = (%v,%v,%v,%v)",
			innerHi, outerHi, innerFilledHi, outerFilledHi, innerOne, outerOne, innerFilledOne, outerFilledOne)
	}
}

func TestUpperBarCell_FullFillIsPlainFullBlock(t *testing.T) {
	glyph, reverse := upperBarCell(1)
	if glyph != '█' || reverse {
		t.Fatalf("upperBarCell(1) = (%q, %v), want ('█', false)", glyph, reverse)
	}
}

func TestUpperBarCell_PartialFillIsReversedComplement(t *testing.T) {
	// A top fill of n/8 must be the bottom-anchored glyph for (8-n)/8 in
	// reverse video, so the visible bar-colored part is the top n/8.
	for k := 0; k < len(barLevels)-1; k++ {
		v := float64(k) / float64(len(barLevels)-1)
		n := k + 1
		glyph, reverse := upperBarCell(v)
		if want := barLevels[len(barLevels)-1-n]; glyph != want || !reverse {
			t.Errorf("upperBarCell(%v) = (%q, %v), want (%q, true) for a %d/8 top fill", v, glyph, reverse, want, n)
		}
	}
}

func TestUpperBarCell_UsesOnlyClassicBlockElements(t *testing.T) {
	// U+2580-U+259F is covered by virtually every monospace font, unlike
	// the Symbols for Legacy Computing block (U+1FB00+) this replaces.
	for i := 0; i <= 100; i++ {
		glyph, _ := upperBarCell(float64(i) / 100)
		if glyph < 0x2580 || glyph > 0x259F {
			t.Fatalf("upperBarCell(%v) = %q (U+%04X), want a Block Elements glyph (U+2580-U+259F)", float64(i)/100, glyph, glyph)
		}
	}
}

func TestClassicUpperBarChar_RoundsToClassicTopAnchoredGlyphs(t *testing.T) {
	for _, tc := range []struct {
		v    float64
		want rune
	}{{0, '▔'}, {0.5, '▀'}, {1, '█'}} {
		if got := classicUpperBarChar(tc.v); got != tc.want {
			t.Errorf("classicUpperBarChar(%v) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

func TestLegacyUpperBarChar_MapsLevelsAcrossRange(t *testing.T) {
	for _, tc := range []struct {
		v    float64
		want rune
	}{{0, '▔'}, {0.5, '🮄'}, {1, '█'}} {
		if got := legacyUpperBarChar(tc.v); got != tc.want {
			t.Errorf("legacyUpperBarChar(%v) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

// withColorProfile sets lipgloss's global color profile for the duration of
// the test, since `go test` output isn't a TTY and defaults to Ascii.
func withColorProfile(t *testing.T, p termenv.Profile) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(p)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
}

func TestRenderVisualizerBox_RendersFourMirroredContentRows(t *testing.T) {
	m := newTestModel()
	m.bands = []float64{0.9, 0.1, 0.5, 0.9, 0.1, 0.5, 0.9, 0.1}
	out := m.renderVisualizerBox(theme.Get("Nord"), 20)

	lines := strings.Split(out, "\n")
	if len(lines) != 6 { // top border + 4 content rows + bottom border
		t.Fatalf("renderVisualizerBox() produced %d lines, want 6 (border+4 content rows+border):\n%s", len(lines), out)
	}
}

func TestRenderVisualizerBox_TopAndBottomRowsMirrorFillPattern(t *testing.T) {
	m := newTestModel()
	m.bands = []float64{0.9, 0.1, 0.5, 0.9, 0.1, 0.5, 0.9, 0.1}
	out := m.renderVisualizerBox(theme.Get("Nord"), 20)

	lines := strings.Split(out, "\n")
	// lines[0]=top border, lines[1]=outer-above, lines[2]=inner-above,
	// lines[3]=inner-below, lines[4]=outer-below, lines[5]=bottom border.
	// Above/below use different (bottom- vs top-anchored) glyph sets so a
	// partial value visually grows away from the center line on both
	// sides — they are NOT expected to be byte-identical (see
	// TestRenderVisualizerBox_BelowCenterRowsUseReverseVideo). What
	// must still match is which columns are blank vs filled, since both
	// sides make that decision from the same splitMirroredLevels() call.
	// Counting literal space runs is a simple, ANSI-escape-safe proxy for
	// "same columns are blank" — styled, filled cells never contain a
	// literal space, and unfilled cells are always exactly one.
	if got, want := strings.Count(lines[1], " "), strings.Count(lines[4], " "); got != want {
		t.Fatalf("outer rows have different blank-column counts: top=%d bottom=%d\n top=%q\n bottom=%q", got, want, lines[1], lines[4])
	}
	if got, want := strings.Count(lines[2], " "), strings.Count(lines[3], " "); got != want {
		t.Fatalf("inner rows have different blank-column counts: top=%d bottom=%d\n top=%q\n bottom=%q", got, want, lines[2], lines[3])
	}
}

func TestRenderVisualizerBox_BelowCenterRowsUseReverseVideo(t *testing.T) {
	withColorProfile(t, termenv.TrueColor)
	m := newTestModel()
	// v=0.3 keeps inner partially filled (fraction 0.6, not maxed) and
	// outer entirely blank, so lines[2]/lines[3] each exercise a partial
	// glyph with no ambiguity from the plain full block '█'.
	m.bands = []float64{0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3}
	out := m.renderVisualizerBox(theme.Get("Nord"), 20)
	lines := strings.Split(out, "\n")

	const reverseVideo = "\x1b[7;"
	if strings.Contains(lines[2], reverseVideo) {
		t.Fatalf("inner-above row (lines[2]) should not use reverse video:\n%q", lines[2])
	}
	if !strings.Contains(lines[3], reverseVideo) {
		t.Fatalf("inner-below row (lines[3]) should draw its top-anchored fill in reverse video:\n%q", lines[3])
	}
	if strings.ContainsAny(lines[3], "🮂🮃🮄🮅🮆") {
		t.Fatalf("inner-below row (lines[3]) should not use Symbols for Legacy Computing glyphs:\n%q", lines[3])
	}
}

func TestRenderVisualizerBox_BelowCenterRowsFallBackWithoutColor(t *testing.T) {
	withColorProfile(t, termenv.Ascii)
	m := newTestModel()
	m.bands = []float64{0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3}
	out := m.renderVisualizerBox(theme.Get("Nord"), 20)
	lines := strings.Split(out, "\n")

	// Reverse video is dropped under Ascii, so a reversed bottom-anchored
	// glyph would draw the wrong shape; only ▔/▀/█ are acceptable here.
	if strings.ContainsAny(lines[3], "▁▂▃▄▅▆▇") {
		t.Fatalf("inner-below row (lines[3]) should not use bottom-anchored glyphs without color support:\n%q", lines[3])
	}
	if !strings.ContainsAny(lines[3], "▔▀") {
		t.Fatalf("inner-below row (lines[3]) should use classic top-anchored glyphs without color support:\n%q", lines[3])
	}
}

func TestRenderVisualizerBox_BelowCenterRowsFollowVisualizerStyle(t *testing.T) {
	withColorProfile(t, termenv.TrueColor)
	const reverseVideo = "\x1b[7;"
	const legacyOnly = "🮂🮃🮄🮅🮆"
	for _, tc := range []struct {
		style       string
		wantReverse bool
		wantAny     string
		wantNone    string
	}{
		{style: "", wantReverse: true, wantAny: "▁▂▃▄▅▆▇", wantNone: legacyOnly},
		{style: "unknown", wantReverse: true, wantAny: "▁▂▃▄▅▆▇", wantNone: legacyOnly},
		{style: visualizerStyleReverse, wantReverse: true, wantAny: "▁▂▃▄▅▆▇", wantNone: legacyOnly},
		// classic and legacy never use reverse video, so the empty part of
		// a cell keeps a transparent terminal background visible.
		{style: visualizerStyleClassic, wantReverse: false, wantAny: "▔▀", wantNone: "▁▂▃▄▅▆▇" + legacyOnly},
		{style: visualizerStyleLegacy, wantReverse: false, wantAny: legacyOnly, wantNone: "▁▂▃▄▅▆▇"},
	} {
		m := newTestModel()
		m.cfg.VisualizerStyle = tc.style
		m.bands = []float64{0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3}
		lines := strings.Split(m.renderVisualizerBox(theme.Get("Nord"), 20), "\n")

		if got := strings.Contains(lines[3], reverseVideo); got != tc.wantReverse {
			t.Errorf("style %q: inner-below row uses reverse video = %v, want %v:\n%q", tc.style, got, tc.wantReverse, lines[3])
		}
		if !strings.ContainsAny(lines[3], tc.wantAny) {
			t.Errorf("style %q: inner-below row should contain one of %q:\n%q", tc.style, tc.wantAny, lines[3])
		}
		if strings.ContainsAny(lines[3], tc.wantNone) {
			t.Errorf("style %q: inner-below row should not contain any of %q:\n%q", tc.style, tc.wantNone, lines[3])
		}
	}
}

func TestRenderVisualizerBox_RenderedWidthMatchesCallerRequest(t *testing.T) {
	m := newTestModel()
	m.bands = []float64{0.9, 0.1, 0.5, 0.9, 0.1, 0.5, 0.9, 0.1}
	for _, width := range []int{20, 40} {
		out := m.renderVisualizerBox(theme.Get("Nord"), width)
		lines := strings.Split(out, "\n")
		if got := len([]rune(lines[0])); got != width+2 {
			t.Errorf("renderVisualizerBox(width=%d) top border rune-width = %d, want %d (width+2 for the border)", width, got, width+2)
		}
	}
}
