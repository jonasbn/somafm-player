package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jonasbn/somafm-player/internal/channels"
	"github.com/jonasbn/somafm-player/internal/config"
	"github.com/jonasbn/somafm-player/internal/history"
	"github.com/jonasbn/somafm-player/internal/player"
)

type channelsFilter int

const (
	filterBookmarked channelsFilter = iota
	filterAll
)

type tunesMode int

const (
	tunesHistory tunesMode = iota
	tunesBookmarked
)

type focusArea int

const (
	focusNowPlaying focusArea = iota
	focusChannels
	focusTunes
)

// defaultWidth is used before the terminal's first tea.WindowSizeMsg
// arrives, so View() never renders against a zero width.
const defaultWidth = 80

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

	channelSelected int
	tuneSelected    int
	channelsFilter  channelsFilter
	tunesMode       tunesMode
	focus           focusArea
	width           int

	player player.Player
	hist   *history.History

	nowPlaying nowPlayingState
	errMsg     string
	quitting   bool

	sessionStarted time.Time
	session        string
}

func New(cfg config.Config, chs []channels.Channel, p player.Player, hist *history.History) Model {
	// Sync the loaded config's volume/mute state into the player itself so
	// the very first Play() call (including the auto-resume path in Init)
	// honors the user's saved settings instead of the player's own defaults.
	p.SetVolume(cfg.Volume)
	p.SetMuted(cfg.Muted)

	filter := filterAll
	if len(cfg.BookmarkedChannels) > 0 {
		filter = filterBookmarked
	}

	return Model{
		cfg:            cfg,
		channels:       chs,
		channelsFilter: filter,
		player:         p,
		hist:           hist,
		width:          defaultWidth,
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

func (m Model) channelsListLen() int {
	if m.channelsFilter == filterAll {
		return len(m.channels)
	}
	return len(m.cfg.BookmarkedChannels)
}

func (m Model) tunesListLen() int {
	if m.tunesMode == tunesHistory {
		return len(m.hist.Entries())
	}
	return len(m.cfg.BookmarkedTunes)
}

func (m Model) toggleChannelsFilter() Model {
	if m.channelsFilter == filterAll {
		m.channelsFilter = filterBookmarked
	} else {
		m.channelsFilter = filterAll
	}
	m.channelSelected = 0
	return m
}

func (m Model) setTunesMode(mode tunesMode) Model {
	m.tunesMode = mode
	m.tuneSelected = 0
	return m
}

func (m Model) selectedChannel() (channels.Channel, bool) {
	switch m.channelsFilter {
	case filterAll:
		if m.channelSelected < len(m.channels) {
			return m.channels[m.channelSelected], true
		}
	case filterBookmarked:
		if m.channelSelected < len(m.cfg.BookmarkedChannels) {
			title := m.cfg.BookmarkedChannels[m.channelSelected]
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyTab:
			switch m.focus {
			case focusNowPlaying:
				m.focus = focusChannels
			case focusChannels:
				m.focus = focusTunes
			case focusTunes:
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
			switch m.focus {
			case focusChannels:
				if n := m.channelsListLen(); n > 0 && m.channelSelected < n-1 {
					m.channelSelected++
				}
			case focusTunes:
				if n := m.tunesListLen(); n > 0 && m.tuneSelected < n-1 {
					m.tuneSelected++
				}
			}
			return m, nil
		case "k", "up":
			switch m.focus {
			case focusChannels:
				if m.channelSelected > 0 {
					m.channelSelected--
				}
			case focusTunes:
				if m.tuneSelected > 0 {
					m.tuneSelected--
				}
			}
			return m, nil
		case "a":
			return m.toggleChannelsFilter(), nil
		case "s":
			return m.setTunesMode(tunesBookmarked), nil
		case "H":
			return m.setTunesMode(tunesHistory), nil
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
		case "r":
			return m, fetchChannelsCmd(channels.DefaultChannelsURL)
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
