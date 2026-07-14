package ui

import (
	"strings"

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

// minSideBySideVisualizerWidth is the content-column floor below which the
// visualizer stops rendering beside Now Playing and falls back to the
// stacked, full-width layout (see view.go's sideBySideVisualizerWidth) —
// set 2 above minDisplayBars so the side-by-side layout gives up before
// bars would get unreadably dense anyway.
const minSideBySideVisualizerWidth = minDisplayBars + 2

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
	if barCount < 0 {
		barCount = 0
	}
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

// splitMirroredLevels maps a 0.0-1.0 band value onto the mirrored waveform
// display's two rows per side: the inner row (touching the implicit center
// line) fills first for v in [0, 0.5] via barChar(v*2); once maxed, the
// outer row fills for the remainder, v in [0.5, 1.0], via
// barChar((v-0.5)*2). innerFilled/outerFilled report whether each row
// should render a glyph at all — v==0 renders both rows blank so silence
// reads as empty space rather than the flat baseline glyph barChar(0)
// would otherwise produce.
func splitMirroredLevels(v float64) (inner rune, outer rune, innerFilled bool, outerFilled bool) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	if v == 0 {
		return 0, 0, false, false
	}
	inner = barChar(v * 2)
	if v <= 0.5 {
		return inner, 0, true, false
	}
	outer = barChar((v - 0.5) * 2)
	return inner, outer, true, true
}

func (m Model) renderVisualizerBox(t theme.Theme, width int) string {
	bars := resampleBands(m.bands, displayBarCount(width))

	var innerRow, outerRow strings.Builder
	for _, v := range bars {
		style := lipgloss.NewStyle().Foreground(gradientColor(v, t))
		inner, outer, innerFilled, outerFilled := splitMirroredLevels(v)
		if innerFilled {
			innerRow.WriteString(style.Render(string(inner)))
		} else {
			innerRow.WriteString(" ")
		}
		if outerFilled {
			outerRow.WriteString(style.Render(string(outer)))
		} else {
			outerRow.WriteString(" ")
		}
	}

	inner := innerRow.String()
	outer := outerRow.String()
	body := strings.Join([]string{outer, inner, inner, outer}, "\n")
	s := borderStyle(t, false)
	return s.Render(body)
}
