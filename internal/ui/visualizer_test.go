package ui

import (
	"math"
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
