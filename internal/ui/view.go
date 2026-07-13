package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jonasbn/somafm-player/internal/bookmarks"
	"github.com/jonasbn/somafm-player/internal/theme"
)

func borderStyle(t theme.Theme, focused bool) lipgloss.Style {
	color := t.Border
	if focused {
		color = t.Accent
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(0, 1)
}

func (m Model) renderNowPlaying(t theme.Theme) string {
	title := m.nowPlaying.title
	if title == "" {
		title = "(nothing playing — press enter on a channel)"
	} else if !m.nowPlaying.connected {
		title = "Reconnecting… (was: " + title + ")"
	}

	line1 := fmt.Sprintf("♪ %s", title)
	if m.nowPlaying.artist != "" {
		line1 += " — " + m.nowPlaying.artist
	}

	line2 := fmt.Sprintf("Channel: %s", m.nowPlaying.channel)
	if m.nowPlaying.bitrate > 0 {
		line2 += fmt.Sprintf("   •   %dk %s", m.nowPlaying.bitrate, m.nowPlaying.codec)
	}

	line3 := fmt.Sprintf("Elapsed: %s   Session: %s", m.nowPlaying.elapsed, m.session)
	line4 := m.renderVolumeBar()

	body := strings.Join([]string{line1, line2, line3, line4}, "\n")
	return borderStyle(t, m.focus == focusNowPlaying).Render(body)
}

func (m Model) renderVolumeBar() string {
	vol := clampVolume(m.cfg.Volume)
	filled := vol / 10
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)
	label := fmt.Sprintf("Vol: %s %d%%", bar, vol)
	if m.cfg.Muted {
		label += " (muted)"
	}
	return label
}

func (m Model) listHeader() string {
	labels := []string{"Channels", "Bookmarked Channels", "Bookmarked Tunes", "History"}
	for i, l := range labels {
		if viewMode(i) == m.mode {
			labels[i] = "[" + l + "]"
		}
	}
	return strings.Join(labels, " ▸ ")
}

func (m Model) listLines() []string {
	switch m.mode {
	case viewChannels:
		lines := make([]string, len(m.channels))
		for i, ch := range m.channels {
			mark := "  "
			if bookmarks.IsChannelBookmarked(m.cfg, ch.Title) {
				mark = "★ "
			}
			lines[i] = fmt.Sprintf("%s%-24s %s", mark, ch.Title, ch.Genre)
		}
		return lines
	case viewBookmarkedChannels:
		lines := make([]string, len(m.cfg.BookmarkedChannels))
		copy(lines, m.cfg.BookmarkedChannels)
		return lines
	case viewBookmarkedTunes:
		lines := make([]string, len(m.cfg.BookmarkedTunes))
		for i, t := range m.cfg.BookmarkedTunes {
			lines[i] = fmt.Sprintf("%s — %s (%s)", t.Title, t.Artist, t.Channel)
		}
		return lines
	case viewHistory:
		entries := m.hist.Entries()
		lines := make([]string, len(entries))
		for i, e := range entries {
			lines[i] = fmt.Sprintf("%s — %s (%s) @ %s", e.Title, e.Artist, e.Channel, e.PlayedAt.Format("15:04:05"))
		}
		return lines
	}
	return nil
}

func (m Model) renderList(t theme.Theme) string {
	lines := m.listLines()
	rendered := make([]string, 0, len(lines)+1)
	rendered = append(rendered, m.listHeader())
	if len(lines) == 0 {
		rendered = append(rendered, "(empty)")
	}
	for i, line := range lines {
		prefix := "  "
		if i == m.selected {
			prefix = "> "
		}
		rendered = append(rendered, prefix+line)
	}
	return borderStyle(t, m.focus == focusList).Render(strings.Join(rendered, "\n"))
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	t := theme.Get(m.cfg.Theme)

	footer := fmt.Sprintf("[Theme: %s]  tab focus · j/k move · enter play · b bookmark · c/f/s/H panels · +/- vol · m mute · t theme · r retry channels · q quit", t.Name)
	if m.errMsg != "" {
		footer = "Error: " + m.errMsg + "\n" + footer
	}

	return strings.Join([]string{
		m.renderNowPlaying(t),
		m.renderList(t),
		footer,
	}, "\n")
}
