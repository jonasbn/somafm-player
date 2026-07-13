package ui

import (
	"testing"

	"github.com/jonasbn/somafm-player/internal/config"
)

func TestBookmarkKey_OnNowPlayingBookmarksCurrentTune(t *testing.T) {
	m := newTestModel()
	m.focus = focusNowPlaying
	m.nowPlaying = nowPlayingState{title: "Track", artist: "Artist", channel: "Groove Salad"}

	m = m.handleBookmarkKey()

	if len(m.cfg.BookmarkedTunes) != 1 || m.cfg.BookmarkedTunes[0].Title != "Track" {
		t.Fatalf("BookmarkedTunes = %+v, want the now-playing tune bookmarked", m.cfg.BookmarkedTunes)
	}
}

func TestBookmarkKey_OnChannelsAllTogglesSelectedChannel(t *testing.T) {
	m := newTestModel()
	m.focus = focusChannels
	m.channelsFilter = filterAll
	m.channelSelected = 0 // "Groove Salad"

	m = m.handleBookmarkKey()
	if len(m.cfg.BookmarkedChannels) != 1 || m.cfg.BookmarkedChannels[0] != "Groove Salad" {
		t.Fatalf("BookmarkedChannels = %v, want [Groove Salad]", m.cfg.BookmarkedChannels)
	}

	m = m.handleBookmarkKey()
	if len(m.cfg.BookmarkedChannels) != 0 {
		t.Fatalf("BookmarkedChannels = %v, want empty after second toggle", m.cfg.BookmarkedChannels)
	}
}

func TestBookmarkKey_OnChannelsBookmarkedTogglesOffSelectedChannel(t *testing.T) {
	m := newTestModel()
	m.cfg.BookmarkedChannels = []string{"Groove Salad", "Drone Zone"}
	m.focus = focusChannels
	m.channelsFilter = filterBookmarked
	m.channelSelected = 1 // "Drone Zone"

	m = m.handleBookmarkKey()

	if len(m.cfg.BookmarkedChannels) != 1 || m.cfg.BookmarkedChannels[0] != "Groove Salad" {
		t.Fatalf("BookmarkedChannels = %v, want [Groove Salad] after removing Drone Zone", m.cfg.BookmarkedChannels)
	}
}

func TestBookmarkKey_OnTunesHistoryBookmarksSelectedEntryAsTune(t *testing.T) {
	m := newTestModel()
	m.focus = focusTunes
	m.tunesMode = tunesHistory
	m.hist.Add(historyEntry("Old Song", "Old Artist", "Drone Zone"))
	m.tuneSelected = 0

	m = m.handleBookmarkKey()

	if len(m.cfg.BookmarkedTunes) != 1 || m.cfg.BookmarkedTunes[0].Title != "Old Song" {
		t.Fatalf("BookmarkedTunes = %+v, want the selected history entry bookmarked", m.cfg.BookmarkedTunes)
	}
}

func TestBookmarkKey_OnTunesBookmarkedIsNoOp(t *testing.T) {
	m := newTestModel()
	m.cfg.BookmarkedTunes = append(m.cfg.BookmarkedTunes, config.BookmarkedTune{Title: "Already Bookmarked"})
	m.focus = focusTunes
	m.tunesMode = tunesBookmarked
	m.tuneSelected = 0

	m = m.handleBookmarkKey()

	if len(m.cfg.BookmarkedTunes) != 1 {
		t.Fatalf("BookmarkedTunes = %+v, want unchanged (no-op) when viewing the bookmarked-tunes list", m.cfg.BookmarkedTunes)
	}
}
