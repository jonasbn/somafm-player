package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jonasbn/somafm-player/internal/bookmarks"
	"github.com/jonasbn/somafm-player/internal/theme"
)

// decorationPerBox accounts for the rounded border (1 col each side) plus
// Padding(0, 1) (1 col each side) that borderStyle applies to every box.
const decorationPerBox = 4

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

func (m Model) channelsHeader() string {
	label := "All"
	if m.channelsFilter == filterBookmarked {
		label = "Bookmarked"
	}
	return fmt.Sprintf("Channels ▸ [%s]  (a) all/bookmarked  (j/k) move", label)
}

func (m Model) channelsLines() []string {
	if m.channelsFilter == filterAll {
		lines := make([]string, len(m.channels))
		for i, ch := range m.channels {
			mark := "  "
			if bookmarks.IsChannelBookmarked(m.cfg, ch.Title) {
				mark = "★ "
			}
			lines[i] = fmt.Sprintf("%s%-24s %s", mark, ch.Title, ch.Genre)
		}
		return lines
	}
	lines := make([]string, len(m.cfg.BookmarkedChannels))
	copy(lines, m.cfg.BookmarkedChannels)
	return lines
}

func (m Model) tunesHeader() string {
	label := "History"
	if m.tunesMode == tunesBookmarked {
		label = "Bookmarked"
	}
	return fmt.Sprintf("Tunes ▸ [%s]  (H/s) history/bookmarked  (j/k) move", label)
}

func (m Model) tunesLines() []string {
	if m.tunesMode == tunesHistory {
		entries := m.hist.Entries()
		lines := make([]string, len(entries))
		for i, e := range entries {
			lines[i] = fmt.Sprintf("%s — %s (%s) @ %s", e.Title, e.Artist, e.Channel, e.PlayedAt.Format("15:04:05"))
		}
		return lines
	}
	lines := make([]string, len(m.cfg.BookmarkedTunes))
	for i, tu := range m.cfg.BookmarkedTunes {
		lines[i] = fmt.Sprintf("%s — %s (%s)", tu.Title, tu.Artist, tu.Channel)
	}
	return lines
}

func renderBox(t theme.Theme, focused bool, width int, header string, lines []string, selected int) string {
	rendered := make([]string, 0, len(lines)+1)
	rendered = append(rendered, header)
	if len(lines) == 0 {
		rendered = append(rendered, "(empty)")
	}
	for i, line := range lines {
		prefix := "  "
		if i == selected {
			prefix = "> "
		}
		rendered = append(rendered, prefix+line)
	}
	return borderStyle(t, focused).Width(width).Render(strings.Join(rendered, "\n"))
}

func (m Model) renderChannelsBox(t theme.Theme, width int) string {
	return renderBox(t, m.focus == focusChannels, width, m.channelsHeader(), m.channelsLines(), m.channelSelected)
}

func (m Model) renderTunesBox(t theme.Theme, width int) string {
	return renderBox(t, m.focus == focusTunes, width, m.tunesHeader(), m.tunesLines(), m.tuneSelected)
}

// boxWidth splits the terminal width evenly between the two side-by-side
// list boxes, minus each box's border/padding decoration. Falls back to
// defaultWidth before the first tea.WindowSizeMsg arrives.
func (m Model) boxWidth() int {
	w := m.width
	if w <= 0 {
		w = defaultWidth
	}
	usable := w - 2*decorationPerBox
	if usable < 2 {
		usable = 2
	}
	return usable / 2
}

// fullBoxWidth returns the available width for a single full-width box
// (unlike boxWidth, which splits the width between two side-by-side
// boxes). Falls back to defaultWidth before the first tea.WindowSizeMsg.
func (m Model) fullBoxWidth() int {
	w := m.width
	if w <= 0 {
		w = defaultWidth
	}
	usable := w - decorationPerBox
	if usable < 2 {
		usable = 2
	}
	return usable
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	t := theme.Get(m.cfg.Theme)

	width := m.boxWidth()
	lists := lipgloss.JoinHorizontal(lipgloss.Top, m.renderChannelsBox(t, width), m.renderTunesBox(t, width))

	sections := []string{m.renderNowPlaying(t)}
	if m.cfg.VisualizerEnabled {
		sections = append(sections, m.renderVisualizerBox(t, m.fullBoxWidth()))
	}
	sections = append(sections, lists)

	footer := fmt.Sprintf("[Theme: %s]  tab focus · j/k move · enter play · b bookmark · a all/bookmarked · H/s tunes · +/- vol · m mute · t theme · v visualizer · r retry channels · q quit", t.Name)
	if m.errMsg != "" {
		footer = "Error: " + m.errMsg + "\n" + footer
	}
	sections = append(sections, footer)

	return strings.Join(sections, "\n")
}
