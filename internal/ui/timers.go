package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func (m Model) handleTick(now time.Time) Model {
	if !m.nowPlaying.trackStarted.IsZero() {
		m.nowPlaying.elapsed = formatDuration(now.Sub(m.nowPlaying.trackStarted))
	}
	if !m.sessionStarted.IsZero() {
		m.session = formatDuration(now.Sub(m.sessionStarted))
	}
	return m
}
