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
