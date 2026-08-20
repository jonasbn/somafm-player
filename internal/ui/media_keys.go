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
