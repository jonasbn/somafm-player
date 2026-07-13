package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jonasbn/somafm-player/internal/channels"
)

// channelsFetchedMsg carries the result of a channel list fetch triggered
// from within the running UI (currently only the "r" retry key).
type channelsFetchedMsg struct {
	channels []channels.Channel
	err      error
}

// fetchChannelsCmd returns a tea.Cmd that fetches the channel list from url
// and reports the outcome as a channelsFetchedMsg. url is a parameter (rather
// than hardcoding channels.DefaultChannelsURL) so it can be pointed at a test
// server in unit tests.
func fetchChannelsCmd(url string) tea.Cmd {
	return func() tea.Msg {
		chs, err := channels.Fetch(context.Background(), url)
		return channelsFetchedMsg{channels: chs, err: err}
	}
}
