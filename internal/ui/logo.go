package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jonasbn/somafm-player/internal/logo"
	"github.com/jonasbn/somafm-player/internal/theme"
)

// logoFallbackText replaces the full ASCII art when the terminal is too
// narrow to fit it without wrapping. Always this literal text, never the
// channel name — the Now Playing box already shows the channel.
const logoFallbackText = "SomaFM"

// logoColor picks the banner's color: the default "somafm" art reads as
// red/warm via each theme's Hot color (already a red-family accent in
// every theme), while channel-specific art uses the theme's normal
// Accent color.
func logoColor(t theme.Theme, isDefault bool) lipgloss.Color {
	if isDefault {
		return t.Hot
	}
	return t.Accent
}

// renderLogo renders the ASCII banner for the currently playing channel
// (or the default "somafm" art when nothing matches), falling back to a
// single-line label below the art's required width. Unlike the other
// panels, it has no border or padding.
func (m Model) renderLogo(t theme.Theme) string {
	lines, isDefault := logo.For(m.nowPlaying.channel)
	style := lipgloss.NewStyle().Foreground(logoColor(t, isDefault))

	w := m.width
	if w <= 0 {
		w = defaultWidth
	}
	if w < logo.Width(lines) {
		return style.Render(logoFallbackText)
	}
	return style.Render(strings.Join(lines, "\n"))
}
