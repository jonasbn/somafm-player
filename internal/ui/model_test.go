package ui

import (
	"testing"

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

func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestUpdate_QuitsOnQ(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(key("q"))
	if cmd == nil || cmd().(tea.QuitMsg) != (tea.QuitMsg{}) {
		t.Fatal("expected q to produce tea.Quit")
	}
}

func TestUpdate_JKMovesSelectionWithinBounds(t *testing.T) {
	m := newTestModel()

	next, _ := m.Update(key("j"))
	m = next.(Model)
	if m.selected != 1 {
		t.Fatalf("selected = %d after j, want 1", m.selected)
	}

	next, _ = m.Update(key("j")) // already at last item, should not overflow
	m = next.(Model)
	if m.selected != 1 {
		t.Fatalf("selected = %d after second j at bottom, want clamped at 1", m.selected)
	}

	next, _ = m.Update(key("k"))
	m = next.(Model)
	if m.selected != 0 {
		t.Fatalf("selected = %d after k, want 0", m.selected)
	}
}

func TestUpdate_PanelSwitchKeysChangeModeAndResetSelection(t *testing.T) {
	m := newTestModel()
	m.selected = 1

	next, _ := m.Update(key("f"))
	m = next.(Model)
	if m.mode != viewBookmarkedChannels || m.selected != 0 {
		t.Fatalf("after f: mode=%v selected=%d, want viewBookmarkedChannels/0", m.mode, m.selected)
	}

	next, _ = m.Update(key("s"))
	m = next.(Model)
	if m.mode != viewBookmarkedTunes {
		t.Fatalf("after s: mode=%v, want viewBookmarkedTunes", m.mode)
	}

	next, _ = m.Update(key("H"))
	m = next.(Model)
	if m.mode != viewHistory {
		t.Fatalf("after H: mode=%v, want viewHistory", m.mode)
	}

	next, _ = m.Update(key("c"))
	m = next.(Model)
	if m.mode != viewChannels {
		t.Fatalf("after c: mode=%v, want viewChannels", m.mode)
	}
}

func TestUpdate_TabTogglesFocus(t *testing.T) {
	m := newTestModel()
	if m.focus != focusNowPlaying {
		t.Fatalf("initial focus = %v, want focusNowPlaying", m.focus)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.focus != focusList {
		t.Fatalf("focus after tab = %v, want focusList", m.focus)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.focus != focusNowPlaying {
		t.Fatalf("focus after second tab = %v, want focusNowPlaying", m.focus)
	}
}
