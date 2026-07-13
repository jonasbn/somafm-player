package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jonasbn/somafm-player/internal/channels"
	"github.com/jonasbn/somafm-player/internal/config"
	"github.com/jonasbn/somafm-player/internal/history"
	"github.com/jonasbn/somafm-player/internal/player"
)

type viewMode int

const (
	viewChannels viewMode = iota
	viewBookmarkedChannels
	viewBookmarkedTunes
	viewHistory
)

type focusArea int

const (
	focusNowPlaying focusArea = iota
	focusList
)

type nowPlayingState struct {
	title        string
	artist       string
	channel      string
	bitrate      int
	codec        string
	connected    bool
	trackStarted time.Time
	elapsed      string
}

type Model struct {
	cfg      config.Config
	channels []channels.Channel
	selected int
	mode     viewMode
	focus    focusArea

	player player.Player
	hist   *history.History

	nowPlaying nowPlayingState
	errMsg     string
	quitting   bool

	sessionStarted time.Time
	session        string
}

func New(cfg config.Config, chs []channels.Channel, p player.Player, hist *history.History) Model {
	return Model{
		cfg:            cfg,
		channels:       chs,
		player:         p,
		hist:           hist,
		sessionStarted: time.Now(),
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{waitForPlayerMsg(m.player), tickCmd()}
	if m.cfg.LastChannel != "" {
		for _, ch := range m.channels {
			if ch.Title == m.cfg.LastChannel {
				cmds = append(cmds, resolveAndPlayCmd(ch))
				break
			}
		}
	}
	return tea.Batch(cmds...)
}

// WithStartupError returns a copy of m with errMsg set, for surfacing a
// startup-time error (e.g. a failed channel list fetch) inline in the UI
// rather than crashing the process.
func (m Model) WithStartupError(msg string) Model {
	m.errMsg = msg
	return m
}

func (m Model) currentListLen() int {
	switch m.mode {
	case viewChannels:
		return len(m.channels)
	case viewBookmarkedChannels:
		return len(m.cfg.BookmarkedChannels)
	case viewBookmarkedTunes:
		return len(m.cfg.BookmarkedTunes)
	case viewHistory:
		return len(m.hist.Entries())
	}
	return 0
}

func (m Model) switchMode(mode viewMode) Model {
	m.mode = mode
	m.selected = 0
	return m
}

func (m Model) selectedChannel() (channels.Channel, bool) {
	switch m.mode {
	case viewChannels:
		if m.selected < len(m.channels) {
			return m.channels[m.selected], true
		}
	case viewBookmarkedChannels:
		if m.selected < len(m.cfg.BookmarkedChannels) {
			title := m.cfg.BookmarkedChannels[m.selected]
			for _, ch := range m.channels {
				if ch.Title == title {
					return ch, true
				}
			}
		}
	}
	return channels.Channel{}, false
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyTab:
			if m.focus == focusNowPlaying {
				m.focus = focusList
			} else {
				m.focus = focusNowPlaying
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			_ = config.Save(m.cfg)
			return m, tea.Quit
		case "j", "down":
			if n := m.currentListLen(); n > 0 && m.selected < n-1 {
				m.selected++
			}
			return m, nil
		case "k", "up":
			if m.selected > 0 {
				m.selected--
			}
			return m, nil
		case "c":
			return m.switchMode(viewChannels), nil
		case "f":
			return m.switchMode(viewBookmarkedChannels), nil
		case "s":
			return m.switchMode(viewBookmarkedTunes), nil
		case "H":
			return m.switchMode(viewHistory), nil
		case "enter":
			if ch, ok := m.selectedChannel(); ok {
				return m, resolveAndPlayCmd(ch)
			}
			return m, nil
		case "+", "right":
			return m.adjustVolume(5), nil
		case "-", "left":
			return m.adjustVolume(-5), nil
		case "m":
			return m.toggleMute(), nil
		case "b":
			return m.handleBookmarkKey(), nil
		case "t":
			return m.cycleTheme(), nil
		}
	}

	if t, ok := msg.(tickMsg); ok {
		return m.handleTick(time.Time(t)), tickCmd()
	}

	switch msg.(type) {
	case player.TrackChangedMsg, player.ConnectionLostMsg, player.ReconnectedMsg:
		next, cmd := m.handlePlaybackMsg(msg)
		return next, tea.Batch(cmd, waitForPlayerMsg(m.player))
	}
	return m.handlePlaybackMsg(msg)
}
