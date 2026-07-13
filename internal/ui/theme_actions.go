package ui

import "github.com/jonasbn/somafm-player/internal/theme"

func (m Model) cycleTheme() Model {
	m.cfg.Theme = theme.Next(m.cfg.Theme)
	return m
}
