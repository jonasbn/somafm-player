package ui

import (
	"strings"
	"testing"

	"github.com/jonasbn/somafm-player/internal/config"
	"github.com/jonasbn/somafm-player/internal/theme"
)

func TestChannelsHeader_ReflectsCurrentFilter(t *testing.T) {
	m := newTestModel()
	m.channelsFilter = filterAll
	if got := m.channelsHeader(); !strings.Contains(got, "[All]") {
		t.Fatalf("channelsHeader() = %q, want it to contain [All]", got)
	}

	m.channelsFilter = filterBookmarked
	if got := m.channelsHeader(); !strings.Contains(got, "[Bookmarked]") {
		t.Fatalf("channelsHeader() = %q, want it to contain [Bookmarked]", got)
	}
}

func TestTunesHeader_ReflectsCurrentMode(t *testing.T) {
	m := newTestModel()
	m.tunesMode = tunesHistory
	if got := m.tunesHeader(); !strings.Contains(got, "[History]") {
		t.Fatalf("tunesHeader() = %q, want it to contain [History]", got)
	}

	m.tunesMode = tunesBookmarked
	if got := m.tunesHeader(); !strings.Contains(got, "[Bookmarked]") {
		t.Fatalf("tunesHeader() = %q, want it to contain [Bookmarked]", got)
	}
}

func TestChannelsLines_AllShowsEveryChannelWithBookmarkMarker(t *testing.T) {
	m := newTestModel()
	m.channelsFilter = filterAll
	m.cfg.BookmarkedChannels = []string{"Drone Zone"}

	lines := m.channelsLines()

	if len(lines) != 2 {
		t.Fatalf("channelsLines() len = %d, want 2", len(lines))
	}
	if strings.Contains(lines[0], "★") {
		t.Fatalf("lines[0] = %q, Groove Salad should not be marked bookmarked", lines[0])
	}
	if !strings.Contains(lines[1], "★") {
		t.Fatalf("lines[1] = %q, Drone Zone should be marked bookmarked", lines[1])
	}
}

func TestChannelsLines_BookmarkedShowsOnlyBookmarkedTitles(t *testing.T) {
	m := newTestModel()
	m.channelsFilter = filterBookmarked
	m.cfg.BookmarkedChannels = []string{"Drone Zone"}

	lines := m.channelsLines()

	if len(lines) != 1 || lines[0] != "Drone Zone" {
		t.Fatalf("channelsLines() = %v, want [Drone Zone]", lines)
	}
}

func TestTunesLines_HistoryShowsPlayedAtTimestamp(t *testing.T) {
	m := newTestModel()
	m.tunesMode = tunesHistory
	m.hist.Add(historyEntry("Song A", "Artist A", "Groove Salad"))

	lines := m.tunesLines()

	if len(lines) != 1 || !strings.HasPrefix(lines[0], "Song A — Artist A (Groove Salad) @ ") {
		t.Fatalf("tunesLines() = %v, want a single History-formatted line", lines)
	}
}

func TestTunesLines_BookmarkedShowsSavedTunes(t *testing.T) {
	m := newTestModel()
	m.tunesMode = tunesBookmarked
	m.cfg.BookmarkedTunes = []config.BookmarkedTune{{Title: "Song B", Artist: "Artist B", Channel: "Drone Zone"}}

	lines := m.tunesLines()

	if len(lines) != 1 || lines[0] != "Song B — Artist B (Drone Zone)" {
		t.Fatalf("tunesLines() = %v, want [\"Song B — Artist B (Drone Zone)\"]", lines)
	}
}

func TestBoxWidth_SplitsWidthEvenlyMinusDecoration(t *testing.T) {
	m := newTestModel()
	m.width = 100

	got := m.boxWidth()

	// 100 total - 2*4 (border+padding decoration per box) = 92, /2 = 46
	if got != 46 {
		t.Fatalf("boxWidth() = %d, want 46 for a 100-column terminal", got)
	}
}

func TestBoxWidth_FallsBackToDefaultWidthWhenZero(t *testing.T) {
	m := newTestModel()
	m.width = 0

	got := m.boxWidth()

	// defaultWidth (80) - 8 = 72, /2 = 36
	if got != 36 {
		t.Fatalf("boxWidth() = %d, want 36 when width is unset", got)
	}
}

func TestFullBoxWidth_SubtractsSingleBoxDecorationOnly(t *testing.T) {
	m := newTestModel()
	m.width = 100

	got := m.fullBoxWidth()

	// 100 total - 4 (border+padding decoration for one box) = 96
	if got != 96 {
		t.Fatalf("fullBoxWidth() = %d, want 96 for a 100-column terminal", got)
	}
}

func TestView_HidesVisualizerBoxByDefault(t *testing.T) {
	m := newTestModel() // config.DefaultConfig() has VisualizerEnabled=false
	out := m.View()
	// Deliberately excludes '█' (U+2588 FULL BLOCK): renderVolumeBar's
	// filled-volume glyph in the Now Playing box already uses '█'
	// unconditionally (config.DefaultConfig().Volume=80 renders 8 of them),
	// so including it here produces a false positive unrelated to the
	// visualizer. '▁'-'▇' are exclusive to renderVisualizerBox/barChar.
	if strings.ContainsAny(out, "▁▂▃▄▅▆▇") {
		t.Fatalf("View() output contains visualizer bar characters while disabled:\n%s", out)
	}
}

func TestView_ShowsVisualizerBoxWhenEnabled(t *testing.T) {
	m := newTestModel()
	m.cfg.VisualizerEnabled = true
	out := m.View()
	if !strings.ContainsAny(out, "▁▂▃▄▅▆▇█") {
		t.Fatalf("View() output missing visualizer bar characters while enabled:\n%s", out)
	}
}

func TestView_FooterMentionsVisualizerKey(t *testing.T) {
	m := newTestModel()
	if got := m.View(); !strings.Contains(got, "v visualizer") {
		t.Fatalf("View() footer missing 'v visualizer' hint:\n%s", got)
	}
}

func TestView_DoesNotPanicWhenQuitting(t *testing.T) {
	m := newTestModel()
	m.quitting = true
	if got := m.View(); got != "" {
		t.Fatalf("View() while quitting = %q, want empty string", got)
	}
}

func TestView_RendersBothBoxesAndFooter(t *testing.T) {
	m := newTestModel()
	out := m.View()

	if !strings.Contains(out, "Channels") || !strings.Contains(out, "Tunes") {
		t.Fatalf("View() output missing Channels/Tunes headers:\n%s", out)
	}
	if !strings.Contains(out, "j/k move") {
		t.Fatalf("View() output missing footer movement hint:\n%s", out)
	}
}

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
	// Populate bands so visualizer renders bars instead of empty space
	m.bands = make([]float64, 32)
	for i := range m.bands {
		m.bands[i] = 0.5 + float64(i%4)*0.125 // Varied levels for visual interest
	}
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

	row := m.renderNowPlayingRow(theme.Get(m.cfg.Theme))
	lines := strings.Split(row, "\n")
	// Now Playing alone renders as 6 lines (border + 4 content rows +
	// border); the visualizer (Task 1) also renders 4 content rows +
	// border = 6 lines. A stacked layout concatenates them (6+6=12 lines);
	// a side-by-side layout joins them onto the same 6 physical rows. This
	// line count is a structural check that doesn't depend on which
	// glyphs land on which row, unlike checking for bar characters on the
	// "♪" line (which the previous version of this test did, and which
	// turned out to depend on band-value/row-alignment coincidences).
	if len(lines) != 12 {
		t.Fatalf("renderNowPlayingRow() at width=20 produced %d lines, want 12 (stacked: Now Playing's 6 lines + visualizer's 6 lines):\n%s", len(lines), row)
	}
}
