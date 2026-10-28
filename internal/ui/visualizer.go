package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"

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

// Visualizer styles, selected by config.Config.VisualizerStyle, control how
// the mirrored waveform's below-center rows draw a top-anchored partial
// fill — a partial value there should visually grow away from the center
// line, the same way barChar does above it. The classic Block Elements set
// only defines two top-anchored fractions (▔ 1/8 and ▀ 4/8), so each style
// trades precision, font coverage and terminal transparency differently.
const (
	// visualizerStyleReverse (the default) draws eighths with the
	// bottom-anchored Block Elements glyphs in reverse video, which any
	// font covers but paints the cell's empty part opaque on terminals
	// with a transparent background.
	visualizerStyleReverse = "reverse"
	// visualizerStyleClassic rounds to ▔/▀/█: works with any font and
	// keeps transparency, at reduced precision.
	visualizerStyleClassic = "classic"
	// visualizerStyleLegacy uses the top-anchored eighths from the
	// "Symbols for Legacy Computing" block (U+1FB82-U+1FB86): full
	// precision without reverse video, but only for fonts or terminals
	// that cover that block (JetBrains Mono, for one, does not).
	visualizerStyleLegacy = "legacy"
)

// legacyUpperBarLevels is barLevels' top-anchored counterpart for
// visualizerStyleLegacy.
var legacyUpperBarLevels = []rune{'▔', '🮂', '🮃', '▀', '🮄', '🮅', '🮆', '█'}

// upperBarCell is barChar's top-anchored counterpart for
// visualizerStyleReverse: a top fill of n/8 is drawn as the bottom-anchored
// glyph for the remaining (8-n)/8 in reverse video, so the glyph's ink
// takes the terminal background and the rest of the cell takes the bar
// color. reverse reports whether the caller must render the glyph that
// way; a full cell is a plain '█'.
func upperBarCell(v float64) (glyph rune, reverse bool) {
	n := topEighths(v)
	if n == len(barLevels) {
		return barLevels[len(barLevels)-1], false
	}
	return barLevels[len(barLevels)-1-n], true
}

// classicUpperBarChar rounds a top-anchored fill to the fractions Block
// Elements does define: ▔ (1/8), ▀ (4/8) and █ (8/8).
func classicUpperBarChar(v float64) rune {
	switch n := topEighths(v); {
	case n <= 2:
		return '▔'
	case n <= 6:
		return '▀'
	default:
		return '█'
	}
}

// legacyUpperBarChar maps a top-anchored fill to legacyUpperBarLevels.
func legacyUpperBarChar(v float64) rune {
	return legacyUpperBarLevels[topEighths(v)-1]
}

// topEighths quantizes a 0.0-1.0 fill level to 1-8 eighths of a cell, the
// same steps barChar uses.
func topEighths(v float64) int {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return int(v*float64(len(barLevels)-1)+0.5) + 1
}

// renderUpperBar renders a top-anchored fill of v in style, according to
// visualizerStyle. The reverse style falls back to classic under the Ascii
// color profile, where termenv drops reverse video along with color and a
// reversed glyph would draw the wrong shape.
func renderUpperBar(style lipgloss.Style, v float64, visualizerStyle string) string {
	switch visualizerStyle {
	case visualizerStyleLegacy:
		return style.Render(string(legacyUpperBarChar(v)))
	case visualizerStyleClassic:
		return style.Render(string(classicUpperBarChar(v)))
	}
	if lipgloss.ColorProfile() == termenv.Ascii {
		return style.Render(string(classicUpperBarChar(v)))
	}
	glyph, reverse := upperBarCell(v)
	return style.Reverse(reverse).Render(string(glyph))
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
// line) fills first for v in [0, 0.5]; once maxed, the outer row fills for
// the remainder, v in [0.5, 1.0]. It returns fill FRACTIONS (0.0-1.0)
// rather than glyphs, since the same fraction renders as a different glyph
// depending on which side of the center line it's drawn on — barChar
// (bottom-anchored) above, renderUpperBar (top-anchored) below — so a
// partial value visually grows away from center on both sides instead of
// the below side just duplicating the above side's shape.
// innerFilled/outerFilled report whether each row should render a glyph at
// all — v==0 reports both rows unfilled so silence reads as empty space
// rather than the flat baseline glyph a fraction of 0 would otherwise
// produce.
func splitMirroredLevels(v float64) (inner float64, outer float64, innerFilled bool, outerFilled bool) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	if v == 0 {
		return 0, 0, false, false
	}
	inner = v * 2
	if inner > 1 {
		inner = 1
	}
	if v <= 0.5 {
		return inner, 0, true, false
	}
	outer = (v - 0.5) * 2
	return inner, outer, true, true
}

func (m Model) renderVisualizerBox(t theme.Theme, width int) string {
	// borderStyle's Padding(0, 1) consumes 2 columns from lipgloss's
	// Width() budget (verified: Style.Width() budgets padding inside the
	// given width, not outside it) — subtract it before sizing bar count,
	// or content overflows the budget and wraps onto extra lines.
	const horizontalPadding = 2
	bars := resampleBands(m.bands, displayBarCount(width-horizontalPadding))

	var innerAbove, outerAbove, innerBelow, outerBelow strings.Builder
	for _, v := range bars {
		style := lipgloss.NewStyle().Foreground(gradientColor(v, t))
		inner, outer, innerFilled, outerFilled := splitMirroredLevels(v)

		if innerFilled {
			innerAbove.WriteString(style.Render(string(barChar(inner))))
			innerBelow.WriteString(renderUpperBar(style, inner, m.cfg.VisualizerStyle))
		} else {
			innerAbove.WriteString(" ")
			innerBelow.WriteString(" ")
		}
		if outerFilled {
			outerAbove.WriteString(style.Render(string(barChar(outer))))
			outerBelow.WriteString(renderUpperBar(style, outer, m.cfg.VisualizerStyle))
		} else {
			outerAbove.WriteString(" ")
			outerBelow.WriteString(" ")
		}
	}

	body := strings.Join([]string{outerAbove.String(), innerAbove.String(), innerBelow.String(), outerBelow.String()}, "\n")
	return borderStyle(t, false).Width(width).Render(body)
}
