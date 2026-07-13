package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/jonasbn/somafm-player/internal/theme"
)

// minDisplayBars/maxDisplayBars bound how many bars the visualizer box
// renders, derived from its available width so it degrades gracefully on
// narrow terminals without getting absurdly dense on ultrawide ones.
const (
	minDisplayBars = 8
	maxDisplayBars = 32
)

var barLevels = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// displayBarCount clamps width to [minDisplayBars, maxDisplayBars].
func displayBarCount(width int) int {
	n := width
	if n < minDisplayBars {
		n = minDisplayBars
	}
	if n > maxDisplayBars {
		n = maxDisplayBars
	}
	return n
}

// resampleBands maps an arbitrary-length bands slice onto barCount output
// values by averaging contiguous groups. A nil or empty bands (nothing
// playing) produces a zero-filled slice of length barCount, rendering as
// flat bars rather than an empty/collapsed box.
func resampleBands(bands []float64, barCount int) []float64 {
	out := make([]float64, barCount)
	if len(bands) == 0 || barCount <= 0 {
		return out
	}
	for i := range out {
		lo := i * len(bands) / barCount
		hi := (i + 1) * len(bands) / barCount
		if hi <= lo {
			hi = lo + 1
		}
		if hi > len(bands) {
			hi = len(bands)
		}
		sum := 0.0
		for j := lo; j < hi; j++ {
			sum += bands[j]
		}
		out[i] = sum / float64(hi-lo)
	}
	return out
}

// barChar maps a 0.0-1.0 fill level to one of 8 sub-cell block characters.
func barChar(v float64) rune {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	idx := int(v*float64(len(barLevels)-1) + 0.5)
	return barLevels[idx]
}

// gradientColor interpolates a bar's color across three stops — Muted (0),
// Accent (0.5), Hot (1) — using perceptual Lab blending so the transition
// reads as smooth rather than muddy.
func gradientColor(v float64, t theme.Theme) lipgloss.Color {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	lo, _ := colorful.Hex(string(t.Muted))
	mid, _ := colorful.Hex(string(t.Accent))
	hi, _ := colorful.Hex(string(t.Hot))

	var c colorful.Color
	if v <= 0.5 {
		c = lo.BlendLab(mid, v/0.5)
	} else {
		c = mid.BlendLab(hi, (v-0.5)/0.5)
	}
	return lipgloss.Color(c.Hex())
}
