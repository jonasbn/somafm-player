package ui

import (
	"github.com/jonasbn/somafm-player/internal/bookmarks"
	"github.com/jonasbn/somafm-player/internal/config"
)

func (m Model) handleBookmarkKey() Model {
	if m.focus == focusNowPlaying {
		if m.nowPlaying.title == "" {
			return m
		}
		bookmarks.AddTune(&m.cfg, config.BookmarkedTune{
			Title:   m.nowPlaying.title,
			Artist:  m.nowPlaying.artist,
			Channel: m.nowPlaying.channel,
		})
		return m
	}

	switch m.mode {
	case viewChannels:
		if ch, ok := m.selectedChannel(); ok {
			bookmarks.ToggleChannel(&m.cfg, ch.Title)
		}
	case viewBookmarkedChannels:
		if m.selected < len(m.cfg.BookmarkedChannels) {
			bookmarks.ToggleChannel(&m.cfg, m.cfg.BookmarkedChannels[m.selected])
		}
	case viewHistory:
		entries := m.hist.Entries()
		if m.selected < len(entries) {
			e := entries[m.selected]
			bookmarks.AddTune(&m.cfg, config.BookmarkedTune{Title: e.Title, Artist: e.Artist, Channel: e.Channel})
		}
	case viewBookmarkedTunes:
		// already bookmarked; no-op
	}
	return m
}
