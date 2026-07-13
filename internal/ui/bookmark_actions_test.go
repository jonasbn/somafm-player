package ui

import "testing"

func TestBookmarkKey_OnNowPlayingBookmarksCurrentTune(t *testing.T) {
	m := newTestModel()
	m.focus = focusNowPlaying
	m.nowPlaying = nowPlayingState{title: "Track", artist: "Artist", channel: "Groove Salad"}

	m = m.handleBookmarkKey()

	if len(m.cfg.BookmarkedTunes) != 1 || m.cfg.BookmarkedTunes[0].Title != "Track" {
		t.Fatalf("BookmarkedTunes = %+v, want the now-playing tune bookmarked", m.cfg.BookmarkedTunes)
	}
}

func TestBookmarkKey_OnChannelsListTogglesSelectedChannel(t *testing.T) {
	m := newTestModel()
	m.focus = focusList
	m.mode = viewChannels
	m.selected = 0 // "Groove Salad"

	m = m.handleBookmarkKey()
	if len(m.cfg.BookmarkedChannels) != 1 || m.cfg.BookmarkedChannels[0] != "Groove Salad" {
		t.Fatalf("BookmarkedChannels = %v, want [Groove Salad]", m.cfg.BookmarkedChannels)
	}

	m = m.handleBookmarkKey()
	if len(m.cfg.BookmarkedChannels) != 0 {
		t.Fatalf("BookmarkedChannels = %v, want empty after second toggle", m.cfg.BookmarkedChannels)
	}
}

func TestBookmarkKey_OnHistoryBookmarksSelectedEntryAsTune(t *testing.T) {
	m := newTestModel()
	m.focus = focusList
	m.mode = viewHistory
	m.hist.Add(historyEntry("Old Song", "Old Artist", "Drone Zone"))
	m.selected = 0

	m = m.handleBookmarkKey()

	if len(m.cfg.BookmarkedTunes) != 1 || m.cfg.BookmarkedTunes[0].Title != "Old Song" {
		t.Fatalf("BookmarkedTunes = %+v, want the selected history entry bookmarked", m.cfg.BookmarkedTunes)
	}
}
