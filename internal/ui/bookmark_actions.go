package ui

import (
	"github.com/jonasbn/somafm-player/internal/bookmarks"
	"github.com/jonasbn/somafm-player/internal/config"
)

func (m Model) handleBookmarkKey() Model {
	switch m.focus {
	case focusNowPlaying:
		if m.nowPlaying.title == "" {
			return m
		}
		bookmarks.AddTune(&m.cfg, config.BookmarkedTune{
			Title:   m.nowPlaying.title,
			Artist:  m.nowPlaying.artist,
			Channel: m.nowPlaying.channel,
		})
	case focusChannels:
		switch m.channelsFilter {
		case filterAll:
			if ch, ok := m.selectedChannel(); ok {
				bookmarks.ToggleChannel(&m.cfg, ch.Title)
			}
		case filterBookmarked:
			if m.channelSelected < len(m.cfg.BookmarkedChannels) {
				bookmarks.ToggleChannel(&m.cfg, m.cfg.BookmarkedChannels[m.channelSelected])
			}
		}
	case focusTunes:
		switch m.tunesMode {
		case tunesHistory:
			entries := m.hist.Entries()
			if m.tuneSelected < len(entries) {
				e := entries[m.tuneSelected]
				bookmarks.AddTune(&m.cfg, config.BookmarkedTune{Title: e.Title, Artist: e.Artist, Channel: e.Channel})
			}
		case tunesBookmarked:
			// already bookmarked; no-op
		}
	}
	return m
}
