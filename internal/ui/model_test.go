package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jonasbn/somafm-player/internal/channels"
	"github.com/jonasbn/somafm-player/internal/config"
	"github.com/jonasbn/somafm-player/internal/history"
	"github.com/jonasbn/somafm-player/internal/player"
)

func newTestModel() Model {
	chs := []channels.Channel{
		{Title: "Groove Salad"},
		{Title: "Drone Zone"},
	}
	return New(config.DefaultConfig(), chs, player.NewFakePlayer(), history.New(5))
}

func newTestModelWithPlayer(p player.Player) Model {
	chs := []channels.Channel{{Title: "Groove Salad"}, {Title: "Drone Zone"}}
	return New(config.DefaultConfig(), chs, p, history.New(5))
}

func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func historyEntry(title, artist, channel string) history.Entry {
	return history.Entry{Title: title, Artist: artist, Channel: channel}
}

func TestUpdate_QuitsOnQ(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := newTestModel()
	_, cmd := m.Update(key("q"))
	if cmd == nil || cmd().(tea.QuitMsg) != (tea.QuitMsg{}) {
		t.Fatal("expected q to produce tea.Quit")
	}
}

func TestNew_DefaultsChannelsFilterToAllWhenNoBookmarks(t *testing.T) {
	m := newTestModel() // config.DefaultConfig() has no BookmarkedChannels
	if m.channelsFilter != filterAll {
		t.Fatalf("channelsFilter = %v, want filterAll when no bookmarks exist", m.channelsFilter)
	}
}

func TestNew_DefaultsChannelsFilterToBookmarkedWhenBookmarksExist(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.BookmarkedChannels = []string{"Groove Salad"}
	chs := []channels.Channel{{Title: "Groove Salad"}, {Title: "Drone Zone"}}

	m := New(cfg, chs, player.NewFakePlayer(), history.New(5))

	if m.channelsFilter != filterBookmarked {
		t.Fatalf("channelsFilter = %v, want filterBookmarked when bookmarks exist", m.channelsFilter)
	}
}

func TestNew_DefaultsWidthTo80(t *testing.T) {
	m := newTestModel()
	if m.width != defaultWidth {
		t.Fatalf("width = %d, want default %d", m.width, defaultWidth)
	}
}

func TestUpdate_WindowSizeMsgStoresWidth(t *testing.T) {
	m := newTestModel()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)
	if m.width != 120 {
		t.Fatalf("width = %d after WindowSizeMsg, want 120", m.width)
	}
}

func TestUpdate_TabCyclesThroughThreeFocusAreas(t *testing.T) {
	m := newTestModel()
	if m.focus != focusNowPlaying {
		t.Fatalf("initial focus = %v, want focusNowPlaying", m.focus)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.focus != focusChannels {
		t.Fatalf("focus after tab = %v, want focusChannels", m.focus)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.focus != focusTunes {
		t.Fatalf("focus after second tab = %v, want focusTunes", m.focus)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.focus != focusNowPlaying {
		t.Fatalf("focus after third tab = %v, want focusNowPlaying", m.focus)
	}
}

func TestUpdate_JKNoOpWhenNowPlayingFocused(t *testing.T) {
	m := newTestModel() // default focus = focusNowPlaying

	next, _ := m.Update(key("j"))
	m = next.(Model)
	if m.channelSelected != 0 || m.tuneSelected != 0 {
		t.Fatalf("channelSelected=%d tuneSelected=%d after j with NowPlaying focused, want both 0", m.channelSelected, m.tuneSelected)
	}
}

func TestUpdate_JKMovesChannelSelectionWithinBounds(t *testing.T) {
	m := newTestModel()
	m.focus = focusChannels // filterAll by default -> 2 channels

	next, _ := m.Update(key("j"))
	m = next.(Model)
	if m.channelSelected != 1 {
		t.Fatalf("channelSelected = %d after j, want 1", m.channelSelected)
	}

	next, _ = m.Update(key("j")) // already at last item, should not overflow
	m = next.(Model)
	if m.channelSelected != 1 {
		t.Fatalf("channelSelected = %d after second j at bottom, want clamped at 1", m.channelSelected)
	}

	next, _ = m.Update(key("k"))
	m = next.(Model)
	if m.channelSelected != 0 {
		t.Fatalf("channelSelected = %d after k, want 0", m.channelSelected)
	}
}

func TestUpdate_JKMovesTuneSelectionWithinBounds(t *testing.T) {
	m := newTestModel()
	m.focus = focusTunes
	m.hist.Add(historyEntry("Song A", "Artist A", "Groove Salad"))
	m.hist.Add(historyEntry("Song B", "Artist B", "Drone Zone"))

	next, _ := m.Update(key("j"))
	m = next.(Model)
	if m.tuneSelected != 1 {
		t.Fatalf("tuneSelected = %d after j, want 1", m.tuneSelected)
	}

	next, _ = m.Update(key("k"))
	m = next.(Model)
	if m.tuneSelected != 0 {
		t.Fatalf("tuneSelected = %d after k, want 0", m.tuneSelected)
	}
}

func TestUpdate_AKeyTogglesChannelsFilterAndResetsSelection(t *testing.T) {
	m := newTestModel() // no bookmarks => default filterAll
	m.channelSelected = 1

	next, _ := m.Update(key("a"))
	m = next.(Model)
	if m.channelsFilter != filterBookmarked || m.channelSelected != 0 {
		t.Fatalf("after a: channelsFilter=%v channelSelected=%d, want filterBookmarked/0", m.channelsFilter, m.channelSelected)
	}

	next, _ = m.Update(key("a"))
	m = next.(Model)
	if m.channelsFilter != filterAll {
		t.Fatalf("after second a: channelsFilter=%v, want filterAll", m.channelsFilter)
	}
}

func TestUpdate_HAndSKeysSetTunesModeAndResetSelection(t *testing.T) {
	m := newTestModel()
	m.tuneSelected = 1

	next, _ := m.Update(key("s"))
	m = next.(Model)
	if m.tunesMode != tunesBookmarked || m.tuneSelected != 0 {
		t.Fatalf("after s: tunesMode=%v tuneSelected=%d, want tunesBookmarked/0", m.tunesMode, m.tuneSelected)
	}

	m.tuneSelected = 1
	next, _ = m.Update(key("H"))
	m = next.(Model)
	if m.tunesMode != tunesHistory || m.tuneSelected != 0 {
		t.Fatalf("after H: tunesMode=%v tuneSelected=%d, want tunesHistory/0", m.tunesMode, m.tuneSelected)
	}
}

func TestUpdate_VisualizerTickMsgReschedulesWhenEnabled(t *testing.T) {
	m := newTestModel()
	m.cfg.VisualizerEnabled = true

	_, cmd := m.Update(visualizerTickMsg(time.Now()))
	if cmd == nil {
		t.Fatal("expected a reschedule cmd when visualizer is enabled")
	}
}

func TestUpdate_VisualizerTickMsgNoOpAndNoRescheduleWhenDisabled(t *testing.T) {
	m := newTestModel()
	m.cfg.VisualizerEnabled = false
	m.bands = []float64{0.5}

	next, cmd := m.Update(visualizerTickMsg(time.Now()))
	m = next.(Model)

	if len(m.bands) != 1 {
		t.Fatalf("bands = %v, want unchanged when disabled", m.bands)
	}
	if cmd != nil {
		t.Fatal("expected no reschedule cmd when visualizer is disabled")
	}
}

func TestInit_SchedulesVisualizerTickWhenEnabledInConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.VisualizerEnabled = true
	m := New(cfg, nil, player.NewFakePlayer(), history.New(5))

	batch, ok := m.Init()().(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init()() = %T, want tea.BatchMsg", m.Init()())
	}
	if len(batch) != 3 {
		t.Fatalf("len(batch) = %d, want 3 (waitForPlayerMsg, tickCmd, visualizerTickCmd)", len(batch))
	}
}

func TestInit_DoesNotScheduleVisualizerTickWhenDisabled(t *testing.T) {
	m := newTestModel() // VisualizerEnabled=false by default

	batch, ok := m.Init()().(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init()() = %T, want tea.BatchMsg", m.Init()())
	}
	if len(batch) != 2 {
		t.Fatalf("len(batch) = %d, want 2 (waitForPlayerMsg, tickCmd)", len(batch))
	}
}

func TestNew_SyncsSavedVolumeAndMuteIntoPlayer(t *testing.T) {
	fp := player.NewFakePlayer()
	cfg := config.DefaultConfig()
	cfg.Volume = 20
	cfg.Muted = true

	_ = New(cfg, nil, fp, history.New(5))

	if got := fp.Volume(); got != 20 {
		t.Fatalf("player volume after New() = %d, want 20 (from saved config)", got)
	}
	if !fp.Muted() {
		t.Fatal("player muted after New() = false, want true (from saved config)")
	}
}
