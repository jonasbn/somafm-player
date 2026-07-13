package ui

import (
	"strings"
	"testing"

	"github.com/jonasbn/somafm-player/internal/config"
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
