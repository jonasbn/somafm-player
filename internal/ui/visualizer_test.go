package ui

import (
	"math"
	"strings"
	"testing"

	"github.com/lucasb-eyer/go-colorful"

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
	out := m.renderVisualizerBox(theme.Get("Nord"), 20)

	lines := strings.Split(out, "\n")
	if len(lines) != 6 { // top border + 4 content rows + bottom border
		t.Fatalf("renderVisualizerBox() produced %d lines, want 6 (border+4 content rows+border):\n%s", len(lines), out)
	}
}

func TestRenderVisualizerBox_TopAndBottomRowsMirrorEachOther(t *testing.T) {
	m := newTestModel()
	m.bands = []float64{0.9, 0.1, 0.5, 0.9, 0.1, 0.5, 0.9, 0.1}
	out := m.renderVisualizerBox(theme.Get("Nord"), 20)

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
