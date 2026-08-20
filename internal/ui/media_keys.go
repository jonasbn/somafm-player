package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jonasbn/somafm-player/internal/mediakeys"
)

// mediaKeyMsg is delivered when the hardware play/pause/toggle key is
// pressed. somafm-player has no true pause for a live stream, so it's
// handled identically to pressing "m": handleMediaKeyMsg toggles mute.
type mediaKeyMsg struct{}

// waitForMediaKeyCmd blocks on the controller's event channel and
// re-arms itself via the same pattern as waitForPlayerMsg: Update's
// mediaKeyMsg case re-issues this command after each event.
func waitForMediaKeyCmd(c mediakeys.Controller) tea.Cmd {
	return func() tea.Msg {
		<-c.Events()
		return mediaKeyMsg{}
	}
}

func (m Model) handleMediaKeyMsg() Model {
	return m.toggleMute()
}

// syncNowPlaying publishes the current channel/title/artist and playing
// state to macOS. Playing is true only when connected and not muted —
// "pause" means mute for a live stream, so that's what macOS should
// reflect too. Called from every path that changes m.nowPlaying or
// m.cfg.Muted.
func (m Model) syncNowPlaying() {
	m.mediaKeys.SetNowPlaying(mediakeys.NowPlayingInfo{
		Channel: m.nowPlaying.channel,
		Title:   m.nowPlaying.title,
		Artist:  m.nowPlaying.artist,
	})
	m.mediaKeys.SetPlaying(m.nowPlaying.connected && !m.cfg.Muted)
}
