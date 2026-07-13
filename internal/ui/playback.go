package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jonasbn/somafm-player/internal/channels"
	"github.com/jonasbn/somafm-player/internal/history"
	"github.com/jonasbn/somafm-player/internal/player"
)

type streamResolvedMsg struct {
	channelTitle string
	streamURL    string
	err          error
}

func resolveAndPlayCmd(ch channels.Channel) tea.Cmd {
	return func() tea.Msg {
		plsURL := ch.BestMP3Stream()
		if plsURL == "" {
			return streamResolvedMsg{channelTitle: ch.Title, err: fmt.Errorf("no MP3 stream available for %s", ch.Title)}
		}
		streamURL, err := channels.ResolveStreamURL(context.Background(), plsURL)
		return streamResolvedMsg{channelTitle: ch.Title, streamURL: streamURL, err: err}
	}
}

func waitForPlayerMsg(p player.Player) tea.Cmd {
	return func() tea.Msg {
		return <-p.Messages()
	}
}

func (m Model) recordCurrentTrackToHistory() Model {
	if m.nowPlaying.title == "" {
		return m
	}
	m.hist.Add(history.Entry{
		Title:    m.nowPlaying.title,
		Artist:   m.nowPlaying.artist,
		Channel:  m.nowPlaying.channel,
		PlayedAt: time.Now(),
	})
	return m
}

func (m Model) handlePlaybackMsg(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case streamResolvedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m = m.recordCurrentTrackToHistory()
		bitrate, codec := channels.ParseBitrateFromURL(msg.streamURL)
		m.nowPlaying = nowPlayingState{
			channel:      msg.channelTitle,
			bitrate:      bitrate,
			codec:        codec,
			connected:    true,
			trackStarted: time.Now(),
		}
		m.cfg.LastChannel = msg.channelTitle
		m.errMsg = ""
		m.player.Play(msg.streamURL)
		return m, nil

	case player.TrackChangedMsg:
		m = m.recordCurrentTrackToHistory()
		m.nowPlaying.title = msg.Title
		m.nowPlaying.artist = msg.Artist
		m.nowPlaying.trackStarted = time.Now()
		return m, nil

	case player.ConnectionLostMsg:
		m.nowPlaying.connected = false
		return m, nil

	case player.ReconnectedMsg:
		m.nowPlaying.connected = true
		return m, nil
	}
	return m, nil
}
