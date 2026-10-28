#!/bin/sh
# Render the characters somafm-player's visualizer uses, so you can check
# which visualizerStyle (reverse, classic or legacy) suits your terminal and
# font. See "Visualizer fonts and terminals" in the README.
#
# Usage: scripts/check-visualizer-glyphs.sh
#
# The script only prints to the terminal; it changes nothing.

# The glyph lists below are space-separated and split on purpose; none of
# the glyphs are glob characters, but disable globbing to be safe.
# shellcheck disable=SC2086
set -f

esc=$(printf '\033')
color="${esc}[36m"           # bar color: cyan
reverse="${esc}[7;36m"       # reverse video with the bar color
reset="${esc}[0m"

# Bottom-anchored eighths (upper half, all styles), 1/8 to 8/8.
upper='▁ ▂ ▃ ▄ ▅ ▆ ▇ █'

# Top-anchored eighths for the lower half, 1/8 to 8/8, per style.
legacy='▔ 🮂 🮃 ▀ 🮄 🮅 🮆 █'
classic='▔ ▔ ▀ ▀ ▀ ▀ █ █'
# reverse: the bottom-anchored glyph for the remaining (8-n)/8, in reverse
# video; a full cell is a plain █.
reverse_glyphs='▇ ▆ ▅ ▄ ▃ ▂ ▁'

# row GLYPHS [STYLE]: print GLYPHS as a rising then falling staircase
# (levels 1-8-1), each glyph wrapped in STYLE, so the shape reads like the
# visualizer's bars.
row() {
	glyphs=$1
	style=${2:-${color}}
	out=''
	set -- $glyphs
	for g in "$@"; do
		out="${out}${style}${g}${reset}"
	done
	# shellcheck disable=SC2046 # word splitting is intended
	set -- $(printf '%s\n' $glyphs | sed '$d' | sed '1!G;h;$!d')
	for g in "$@"; do
		out="${out}${style}${g}${reset}"
	done
	printf '   %s\n' "${out}"
}

# reverse_row: the reverse style's lower-half staircase, which mixes
# reversed glyphs with a plain full block at the peak.
reverse_row() {
	out=''
	for g in $reverse_glyphs; do
		out="${out}${reverse}${g}${reset}"
	done
	out="${out}${color}█${reset}"
	for g in $(printf '%s\n' $reverse_glyphs | sed '1!G;h;$!d'); do
		out="${out}${reverse}${g}${reset}"
	done
	printf '   %s\n' "${out}"
}

cat <<EOF
somafm-player visualizer glyph check
=====================================

Each sample below is a mirrored waveform: a staircase rising from 1/8 to a
full cell and back down, drawn above and below a center line. In a working
sample the two rows meet at the center line with no gap, and the lower row is
an exact upside-down copy of the upper row.

EOF

printf '1. reverse (default): works with any font\n\n'
row "$upper"
reverse_row
cat <<EOF

   If the lower row shows solid background-colored patches over a
   transparent terminal background, use "classic" or "legacy" instead.

EOF

printf '2. classic: works with any font and with transparency, three steps only\n\n'
row "$upper"
row "$classic"
cat <<EOF

   The lower row is deliberately coarser than the upper row.

EOF

printf '3. legacy: needs the Unicode 13 "Symbols for Legacy Computing" characters\n\n'
row "$upper"
row "$legacy"
cat <<EOF

   If the lower row has blanks, boxes or question marks, your font and
   terminal can't draw these characters; use "reverse" or "classic".

   The characters individually:

EOF

for entry in '1FB82 🮂 2/8' '1FB83 🮃 3/8' '1FB84 🮄 5/8' '1FB85 🮅 6/8' '1FB86 🮆 7/8'; do
	set -- $entry
	printf '     U+%s  [%s%s%s]  upper %s of the cell\n' "$1" "${color}" "$2" "${reset}" "$3"
done

cat <<EOF

Pick the best-looking sample that works, then set it as "reverse",
"classic" or "legacy" in ~/.config/somafm-player/config.json (or under
\$XDG_CONFIG_HOME), for example:

    "visualizerStyle": "classic"
EOF
