# VHS tape for the SomaFM Player README demo.
#
# Regenerate with (from the repository root):
#   vhs docs/demo.tape
#
# Requires the vhs CLI (https://github.com/charmbracelet/vhs) and its
# dependencies (ttyd, ffmpeg) on PATH. The recording runs the real binary
# end-to-end, so it also needs live network access (to fetch SomaFM's
# channel list and stream audio) and a working audio output device.
#
# The demo seeds an isolated config (via XDG_CONFIG_HOME) so it never reads
# or overwrites a real ~/.config/somafm-player/config.json — see the
# "Test isolation" gotcha in CLAUDE.md for why that matters.
#
# Flow: play "Drone Zone" (channel-specific logo), switch to "Groove Salad"
# (default logo), then cycle through a couple of themes.

Output docs/demo.gif

Require go

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 720
Set Theme "Dracula"
Set Padding 20
Set TypingSpeed 60ms

Hide
Type "export XDG_CONFIG_HOME=$(mktemp -d)" Enter
Type "mkdir -p $XDG_CONFIG_HOME/somafm-player" Enter
Type 'cat > $XDG_CONFIG_HOME/somafm-player/config.json <<"EOF"' Enter
Type '{"lastChannel": "", "volume": 45, "muted": false, "theme": "Nord", "bookmarkedChannels": ["Drone Zone", "Groove Salad"], "bookmarkedTunes": [], "visualizerEnabled": true}' Enter
Type "EOF" Enter
Type "go build -o /tmp/somafm-player-demo ." Enter
Sleep 10s
Type "clear" Enter
Show

Type "/tmp/somafm-player-demo"
Enter
Sleep 2s

# Focus moves Now Playing -> Channels -> Tunes; one Tab reaches Channels.
Tab
Sleep 500ms

# The seeded config bookmarks two channels, so the Channels panel already
# shows just these two, with "Drone Zone" selected first.
Enter
Sleep 4s

# Move to the second bookmarked channel ("Groove Salad") and play it —
# the channel-specific Drone Zone logo reverts to the default "somafm" logo.
Down
Sleep 500ms
Enter
Sleep 4s

# Cycle a couple of themes.
Type "t"
Sleep 1s
Type "t"
Sleep 1s

Sleep 2s
Type "q"
