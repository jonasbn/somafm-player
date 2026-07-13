# Channels/Tunes Panel Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the single tabbed List box in the somafm-player TUI into two always-visible boxes (Channels and Tunes), add an `a` all/bookmarked toggle for Channels, and improve `j`/`k` discoverability.

**Architecture:** Replace the single `viewMode` enum and shared `selected`/`focus(List)` state in `internal/ui/model.go` with two independent per-box states (`channelsFilter`, `tunesMode`) and per-box selection indices, a three-way `focusArea`, and terminal-width tracking via `tea.WindowSizeMsg`. `internal/ui/view.go` renders the two boxes side by side with `lipgloss.JoinHorizontal`. `internal/ui/bookmark_actions.go` is updated to branch on the new focus/filter/mode values instead of the old `mode` enum.

**Tech Stack:** Go 1.26, Bubble Tea (`github.com/charmbracelet/bubbletea`), Lip Gloss (`github.com/charmbracelet/lipgloss`).

## Global Constraints

- Any test touching the quit-key path or `config.Save`/`config.Load` MUST call `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` first (see `internal/config/config_test.go`).
- Run `go mod tidy` after adding any new direct import (none expected in this plan — no new dependencies).
- Format all Go files with `gofmt` before committing.
- No TTY/audio hardware in the agent environment — final interactive/visual confirmation must be done by a human running `go run .` in a real terminal; this plan's automated steps cover everything that's unit-testable (pure state transitions and pure render-helper string output).

---

### Task 1: Model state refactor — filters, focus, selection, window width

**Files:**
- Modify: `internal/ui/model.go` (full replacement of enum/struct/method/`Update()` sections)
- Modify: `internal/ui/model_test.go` (full replacement)

**Interfaces:**
- Consumes: `config.Config` (`BookmarkedChannels []string`, `BookmarkedTunes []config.BookmarkedTune`), `channels.Channel{Title, Genre string}`, `history.History.Entries() []history.Entry`, `player.Player` (unchanged).
- Produces (used by Task 2 and Task 3):
  - Type `channelsFilter int` with values `filterBookmarked`, `filterAll`
  - Type `tunesMode int` with values `tunesHistory`, `tunesBookmarked`
  - Type `focusArea int` with values `focusNowPlaying`, `focusChannels`, `focusTunes`
  - `Model` fields: `channelSelected int`, `tuneSelected int`, `channelsFilter channelsFilter`, `tunesMode tunesMode`, `focus focusArea`, `width int`
  - `func (m Model) channelsListLen() int`
  - `func (m Model) tunesListLen() int`
  - `func (m Model) toggleChannelsFilter() Model`
  - `func (m Model) setTunesMode(mode tunesMode) Model`
  - `func (m Model) selectedChannel() (channels.Channel, bool)`
  - Constant `defaultWidth = 80`

- [ ] **Step 1: Write the new/updated tests in `internal/ui/model_test.go`**

```go
package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jonasbn/somafm-player/internal/channels"
	"github.com/jonasbn/somafm-player/internal/config"
	"github.com/jonasbn/somafm-player/internal/history"
	"github.com/jonasbn/somafm-player/internal/player"
)

func newTestModel() Model {
	chs := []channels.Channel{
		{Title: "Groove Salad"},
		{Title: "Drone Zone"},
	}
	return New(config.DefaultConfig(), chs, player.NewFakePlayer(), history.New(5))
}

func newTestModelWithPlayer(p player.Player) Model {
	chs := []channels.Channel{{Title: "Groove Salad"}, {Title: "Drone Zone"}}
	return New(config.DefaultConfig(), chs, p, history.New(5))
}

func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func historyEntry(title, artist, channel string) history.Entry {
	return history.Entry{Title: title, Artist: artist, Channel: channel}
}

func TestUpdate_QuitsOnQ(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := newTestModel()
	_, cmd := m.Update(key("q"))
	if cmd == nil || cmd().(tea.QuitMsg) != (tea.QuitMsg{}) {
		t.Fatal("expected q to produce tea.Quit")
	}
}

func TestNew_DefaultsChannelsFilterToAllWhenNoBookmarks(t *testing.T) {
	m := newTestModel() // config.DefaultConfig() has no BookmarkedChannels
	if m.channelsFilter != filterAll {
		t.Fatalf("channelsFilter = %v, want filterAll when no bookmarks exist", m.channelsFilter)
	}
}

func TestNew_DefaultsChannelsFilterToBookmarkedWhenBookmarksExist(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.BookmarkedChannels = []string{"Groove Salad"}
	chs := []channels.Channel{{Title: "Groove Salad"}, {Title: "Drone Zone"}}

	m := New(cfg, chs, player.NewFakePlayer(), history.New(5))

	if m.channelsFilter != filterBookmarked {
		t.Fatalf("channelsFilter = %v, want filterBookmarked when bookmarks exist", m.channelsFilter)
	}
}

func TestNew_DefaultsWidthTo80(t *testing.T) {
	m := newTestModel()
	if m.width != defaultWidth {
		t.Fatalf("width = %d, want default %d", m.width, defaultWidth)
	}
}

func TestUpdate_WindowSizeMsgStoresWidth(t *testing.T) {
	m := newTestModel()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)
	if m.width != 120 {
		t.Fatalf("width = %d after WindowSizeMsg, want 120", m.width)
	}
}

func TestUpdate_TabCyclesThroughThreeFocusAreas(t *testing.T) {
	m := newTestModel()
	if m.focus != focusNowPlaying {
		t.Fatalf("initial focus = %v, want focusNowPlaying", m.focus)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.focus != focusChannels {
		t.Fatalf("focus after tab = %v, want focusChannels", m.focus)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.focus != focusTunes {
		t.Fatalf("focus after second tab = %v, want focusTunes", m.focus)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.focus != focusNowPlaying {
		t.Fatalf("focus after third tab = %v, want focusNowPlaying", m.focus)
	}
}

func TestUpdate_JKNoOpWhenNowPlayingFocused(t *testing.T) {
	m := newTestModel() // default focus = focusNowPlaying

	next, _ := m.Update(key("j"))
	m = next.(Model)
	if m.channelSelected != 0 || m.tuneSelected != 0 {
		t.Fatalf("channelSelected=%d tuneSelected=%d after j with NowPlaying focused, want both 0", m.channelSelected, m.tuneSelected)
	}
}

func TestUpdate_JKMovesChannelSelectionWithinBounds(t *testing.T) {
	m := newTestModel()
	m.focus = focusChannels // filterAll by default -> 2 channels

	next, _ := m.Update(key("j"))
	m = next.(Model)
	if m.channelSelected != 1 {
		t.Fatalf("channelSelected = %d after j, want 1", m.channelSelected)
	}

	next, _ = m.Update(key("j")) // already at last item, should not overflow
	m = next.(Model)
	if m.channelSelected != 1 {
		t.Fatalf("channelSelected = %d after second j at bottom, want clamped at 1", m.channelSelected)
	}

	next, _ = m.Update(key("k"))
	m = next.(Model)
	if m.channelSelected != 0 {
		t.Fatalf("channelSelected = %d after k, want 0", m.channelSelected)
	}
}

func TestUpdate_JKMovesTuneSelectionWithinBounds(t *testing.T) {
	m := newTestModel()
	m.focus = focusTunes
	m.hist.Add(historyEntry("Song A", "Artist A", "Groove Salad"))
	m.hist.Add(historyEntry("Song B", "Artist B", "Drone Zone"))

	next, _ := m.Update(key("j"))
	m = next.(Model)
	if m.tuneSelected != 1 {
		t.Fatalf("tuneSelected = %d after j, want 1", m.tuneSelected)
	}

	next, _ = m.Update(key("k"))
	m = next.(Model)
	if m.tuneSelected != 0 {
		t.Fatalf("tuneSelected = %d after k, want 0", m.tuneSelected)
	}
}

func TestUpdate_AKeyTogglesChannelsFilterAndResetsSelection(t *testing.T) {
	m := newTestModel() // no bookmarks => default filterAll
	m.channelSelected = 1

	next, _ := m.Update(key("a"))
	m = next.(Model)
	if m.channelsFilter != filterBookmarked || m.channelSelected != 0 {
		t.Fatalf("after a: channelsFilter=%v channelSelected=%d, want filterBookmarked/0", m.channelsFilter, m.channelSelected)
	}

	next, _ = m.Update(key("a"))
	m = next.(Model)
	if m.channelsFilter != filterAll {
		t.Fatalf("after second a: channelsFilter=%v, want filterAll", m.channelsFilter)
	}
}

func TestUpdate_HAndSKeysSetTunesModeAndResetSelection(t *testing.T) {
	m := newTestModel()
	m.tuneSelected = 1

	next, _ := m.Update(key("s"))
	m = next.(Model)
	if m.tunesMode != tunesBookmarked || m.tuneSelected != 0 {
		t.Fatalf("after s: tunesMode=%v tuneSelected=%d, want tunesBookmarked/0", m.tunesMode, m.tuneSelected)
	}

	m.tuneSelected = 1
	next, _ = m.Update(key("H"))
	m = next.(Model)
	if m.tunesMode != tunesHistory || m.tuneSelected != 0 {
		t.Fatalf("after H: tunesMode=%v tuneSelected=%d, want tunesHistory/0", m.tunesMode, m.tuneSelected)
	}
}

func TestNew_SyncsSavedVolumeAndMuteIntoPlayer(t *testing.T) {
	fp := player.NewFakePlayer()
	cfg := config.DefaultConfig()
	cfg.Volume = 20
	cfg.Muted = true

	_ = New(cfg, nil, fp, history.New(5))

	if got := fp.Volume(); got != 20 {
		t.Fatalf("player volume after New() = %d, want 20 (from saved config)", got)
	}
	if !fp.Muted() {
		t.Fatal("player muted after New() = false, want true (from saved config)")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/... -run 'TestNew_DefaultsChannelsFilter|TestUpdate_TabCyclesThroughThreeFocusAreas|TestUpdate_AKeyTogglesChannelsFilter' -v`
Expected: build failure — `undefined: filterAll`, `undefined: focusChannels`, etc. (the old `model.go` doesn't define these identifiers yet).

- [ ] **Step 3: Replace `internal/ui/model.go`**

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ui/... -v`
Expected: all tests in `model_test.go` PASS. (`view.go` and `bookmark_actions.go` will fail to compile at this point — that's expected until Tasks 2 and 3 land; if you need `model_test.go` to compile standalone before then, proceed straight to Task 2, since these three files are interdependent and the package won't build until all three are updated.)

- [ ] **Step 5: Commit**

```bash
gofmt -l internal/ui/model.go internal/ui/model_test.go
git add internal/ui/model.go internal/ui/model_test.go
git commit -m "refactor: split channel/tune view state into independent focus areas"
```

---

### Task 2: Bookmark key handling for the two independent boxes

**Files:**
- Modify: `internal/ui/bookmark_actions.go` (full replacement)
- Modify: `internal/ui/bookmark_actions_test.go` (full replacement)

**Interfaces:**
- Consumes (from Task 1): `Model.focus focusArea` (`focusNowPlaying`, `focusChannels`, `focusTunes`), `Model.channelsFilter channelsFilter` (`filterAll`, `filterBookmarked`), `Model.tunesMode tunesMode` (`tunesHistory`, `tunesBookmarked`), `Model.channelSelected int`, `Model.tuneSelected int`, `Model.selectedChannel() (channels.Channel, bool)`.
- Produces: `func (m Model) handleBookmarkKey() Model` (same signature as before — called from `model.go`'s `Update()` on the `"b"` key, unchanged call site).

- [ ] **Step 1: Write the updated tests in `internal/ui/bookmark_actions_test.go`**

```go
package ui

import "testing"

func TestBookmarkKey_OnNowPlayingBookmarksCurrentTune(t *testing.T) {
	m := newTestModel()
	m.focus = focusNowPlaying
	m.nowPlaying = nowPlayingState{title: "Track", artist: "Artist", channel: "Groove Salad"}

	m = m.handleBookmarkKey()

	if len(m.cfg.BookmarkedTunes) != 1 || m.cfg.BookmarkedTunes[0].Title != "Track" {
		t.Fatalf("BookmarkedTunes = %+v, want the now-playing tune bookmarked", m.cfg.BookmarkedTunes)
	}
}

func TestBookmarkKey_OnChannelsAllTogglesSelectedChannel(t *testing.T) {
	m := newTestModel()
	m.focus = focusChannels
	m.channelsFilter = filterAll
	m.channelSelected = 0 // "Groove Salad"

	m = m.handleBookmarkKey()
	if len(m.cfg.BookmarkedChannels) != 1 || m.cfg.BookmarkedChannels[0] != "Groove Salad" {
		t.Fatalf("BookmarkedChannels = %v, want [Groove Salad]", m.cfg.BookmarkedChannels)
	}

	m = m.handleBookmarkKey()
	if len(m.cfg.BookmarkedChannels) != 0 {
		t.Fatalf("BookmarkedChannels = %v, want empty after second toggle", m.cfg.BookmarkedChannels)
	}
}

func TestBookmarkKey_OnChannelsBookmarkedTogglesOffSelectedChannel(t *testing.T) {
	m := newTestModel()
	m.cfg.BookmarkedChannels = []string{"Groove Salad", "Drone Zone"}
	m.focus = focusChannels
	m.channelsFilter = filterBookmarked
	m.channelSelected = 1 // "Drone Zone"

	m = m.handleBookmarkKey()

	if len(m.cfg.BookmarkedChannels) != 1 || m.cfg.BookmarkedChannels[0] != "Groove Salad" {
		t.Fatalf("BookmarkedChannels = %v, want [Groove Salad] after removing Drone Zone", m.cfg.BookmarkedChannels)
	}
}

func TestBookmarkKey_OnTunesHistoryBookmarksSelectedEntryAsTune(t *testing.T) {
	m := newTestModel()
	m.focus = focusTunes
	m.tunesMode = tunesHistory
	m.hist.Add(historyEntry("Old Song", "Old Artist", "Drone Zone"))
	m.tuneSelected = 0

	m = m.handleBookmarkKey()

	if len(m.cfg.BookmarkedTunes) != 1 || m.cfg.BookmarkedTunes[0].Title != "Old Song" {
		t.Fatalf("BookmarkedTunes = %+v, want the selected history entry bookmarked", m.cfg.BookmarkedTunes)
	}
}

func TestBookmarkKey_OnTunesBookmarkedIsNoOp(t *testing.T) {
	m := newTestModel()
	m.cfg.BookmarkedTunes = append(m.cfg.BookmarkedTunes, config.BookmarkedTune{Title: "Already Bookmarked"})
	m.focus = focusTunes
	m.tunesMode = tunesBookmarked
	m.tuneSelected = 0

	m = m.handleBookmarkKey()

	if len(m.cfg.BookmarkedTunes) != 1 {
		t.Fatalf("BookmarkedTunes = %+v, want unchanged (no-op) when viewing the bookmarked-tunes list", m.cfg.BookmarkedTunes)
	}
}
```

Add `"github.com/jonasbn/somafm-player/internal/config"` to the import block for that test. Final import block:

```go
import (
	"testing"

	"github.com/jonasbn/somafm-player/internal/config"
)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/... -run TestBookmarkKey -v`
Expected: build failure — `handleBookmarkKey` still references the old `m.mode`/`m.selected` fields removed in Task 1, so the package fails to compile.

- [ ] **Step 3: Replace `internal/ui/bookmark_actions.go`**

```go
package ui

import (
	"github.com/jonasbn/somafm-player/internal/bookmarks"
	"github.com/jonasbn/somafm-player/internal/config"
)

func (m Model) handleBookmarkKey() Model {
	switch m.focus {
	case focusNowPlaying:
		if m.nowPlaying.title == "" {
			return m
		}
		bookmarks.AddTune(&m.cfg, config.BookmarkedTune{
			Title:   m.nowPlaying.title,
			Artist:  m.nowPlaying.artist,
			Channel: m.nowPlaying.channel,
		})
	case focusChannels:
		switch m.channelsFilter {
		case filterAll:
			if ch, ok := m.selectedChannel(); ok {
				bookmarks.ToggleChannel(&m.cfg, ch.Title)
			}
		case filterBookmarked:
			if m.channelSelected < len(m.cfg.BookmarkedChannels) {
				bookmarks.ToggleChannel(&m.cfg, m.cfg.BookmarkedChannels[m.channelSelected])
			}
		}
	case focusTunes:
		switch m.tunesMode {
		case tunesHistory:
			entries := m.hist.Entries()
			if m.tuneSelected < len(entries) {
				e := entries[m.tuneSelected]
				bookmarks.AddTune(&m.cfg, config.BookmarkedTune{Title: e.Title, Artist: e.Artist, Channel: e.Channel})
			}
		case tunesBookmarked:
			// already bookmarked; no-op
		}
	}
	return m
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ui/... -run TestBookmarkKey -v`
Expected: all `TestBookmarkKey_*` tests PASS. (Full package build still requires Task 3's `view.go` update.)

- [ ] **Step 5: Commit**

```bash
gofmt -l internal/ui/bookmark_actions.go internal/ui/bookmark_actions_test.go
git add internal/ui/bookmark_actions.go internal/ui/bookmark_actions_test.go
git commit -m "refactor: branch bookmark handling on the new channels/tunes focus state"
```

---

### Task 3: Side-by-side Channels/Tunes rendering

**Files:**
- Modify: `internal/ui/view.go` (full replacement)
- Create: `internal/ui/view_test.go`

**Interfaces:**
- Consumes (from Task 1): `Model.focus`, `Model.channelsFilter`, `Model.tunesMode`, `Model.channelSelected`, `Model.tuneSelected`, `Model.width`, `Model.channels`, `Model.cfg`, `Model.hist`, `defaultWidth` constant.
- Produces: `func (m Model) View() string` (unchanged public signature — Bubble Tea's `tea.Model` interface requirement), plus internal helpers `channelsHeader`, `channelsLines`, `tunesHeader`, `tunesLines`, `boxWidth` used only within `view.go`.

- [ ] **Step 1: Write `internal/ui/view_test.go`**

```go
package ui

import (
	"strings"
	"testing"

	"github.com/jonasbn/somafm-player/internal/config"
)

func TestChannelsHeader_ReflectsCurrentFilter(t *testing.T) {
	m := newTestModel()
	m.channelsFilter = filterAll
	if got := m.channelsHeader(); !strings.Contains(got, "[All]") {
		t.Fatalf("channelsHeader() = %q, want it to contain [All]", got)
	}

	m.channelsFilter = filterBookmarked
	if got := m.channelsHeader(); !strings.Contains(got, "[Bookmarked]") {
		t.Fatalf("channelsHeader() = %q, want it to contain [Bookmarked]", got)
	}
}

func TestTunesHeader_ReflectsCurrentMode(t *testing.T) {
	m := newTestModel()
	m.tunesMode = tunesHistory
	if got := m.tunesHeader(); !strings.Contains(got, "[History]") {
		t.Fatalf("tunesHeader() = %q, want it to contain [History]", got)
	}

	m.tunesMode = tunesBookmarked
	if got := m.tunesHeader(); !strings.Contains(got, "[Bookmarked]") {
		t.Fatalf("tunesHeader() = %q, want it to contain [Bookmarked]", got)
	}
}

func TestChannelsLines_AllShowsEveryChannelWithBookmarkMarker(t *testing.T) {
	m := newTestModel()
	m.channelsFilter = filterAll
	m.cfg.BookmarkedChannels = []string{"Drone Zone"}

	lines := m.channelsLines()

	if len(lines) != 2 {
		t.Fatalf("channelsLines() len = %d, want 2", len(lines))
	}
	if strings.Contains(lines[0], "★") {
		t.Fatalf("lines[0] = %q, Groove Salad should not be marked bookmarked", lines[0])
	}
	if !strings.Contains(lines[1], "★") {
		t.Fatalf("lines[1] = %q, Drone Zone should be marked bookmarked", lines[1])
	}
}

func TestChannelsLines_BookmarkedShowsOnlyBookmarkedTitles(t *testing.T) {
	m := newTestModel()
	m.channelsFilter = filterBookmarked
	m.cfg.BookmarkedChannels = []string{"Drone Zone"}

	lines := m.channelsLines()

	if len(lines) != 1 || lines[0] != "Drone Zone" {
		t.Fatalf("channelsLines() = %v, want [Drone Zone]", lines)
	}
}

func TestTunesLines_HistoryShowsPlayedAtTimestamp(t *testing.T) {
	m := newTestModel()
	m.tunesMode = tunesHistory
	m.hist.Add(historyEntry("Song A", "Artist A", "Groove Salad"))

	lines := m.tunesLines()

	if len(lines) != 1 || !strings.HasPrefix(lines[0], "Song A — Artist A (Groove Salad) @ ") {
		t.Fatalf("tunesLines() = %v, want a single History-formatted line", lines)
	}
}

func TestTunesLines_BookmarkedShowsSavedTunes(t *testing.T) {
	m := newTestModel()
	m.tunesMode = tunesBookmarked
	m.cfg.BookmarkedTunes = []config.BookmarkedTune{{Title: "Song B", Artist: "Artist B", Channel: "Drone Zone"}}

	lines := m.tunesLines()

	if len(lines) != 1 || lines[0] != "Song B — Artist B (Drone Zone)" {
		t.Fatalf("tunesLines() = %v, want [\"Song B — Artist B (Drone Zone)\"]", lines)
	}
}

func TestBoxWidth_SplitsWidthEvenlyMinusDecoration(t *testing.T) {
	m := newTestModel()
	m.width = 100

	got := m.boxWidth()

	// 100 total - 2*4 (border+padding decoration per box) = 92, /2 = 46
	if got != 46 {
		t.Fatalf("boxWidth() = %d, want 46 for a 100-column terminal", got)
	}
}

func TestBoxWidth_FallsBackToDefaultWidthWhenZero(t *testing.T) {
	m := newTestModel()
	m.width = 0

	got := m.boxWidth()

	// defaultWidth (80) - 8 = 72, /2 = 36
	if got != 36 {
		t.Fatalf("boxWidth() = %d, want 36 when width is unset", got)
	}
}

func TestView_DoesNotPanicWhenQuitting(t *testing.T) {
	m := newTestModel()
	m.quitting = true
	if got := m.View(); got != "" {
		t.Fatalf("View() while quitting = %q, want empty string", got)
	}
}

func TestView_RendersBothBoxesAndFooter(t *testing.T) {
	m := newTestModel()
	out := m.View()

	if !strings.Contains(out, "Channels") || !strings.Contains(out, "Tunes") {
		t.Fatalf("View() output missing Channels/Tunes headers:\n%s", out)
	}
	if !strings.Contains(out, "j/k move") {
		t.Fatalf("View() output missing footer movement hint:\n%s", out)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/... -run 'TestChannelsHeader|TestTunesHeader|TestChannelsLines|TestTunesLines|TestBoxWidth|TestView_' -v`
Expected: build failure — `channelsHeader`, `tunesHeader`, `channelsLines`, `tunesLines`, `boxWidth` are not defined on `Model` yet (old `view.go` still has `listHeader`/`listLines`/`renderList` referencing the removed `mode`/`selected` fields, so the package won't even compile).

- [ ] **Step 3: Replace `internal/ui/view.go`**

```go
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jonasbn/somafm-player/internal/bookmarks"
	"github.com/jonasbn/somafm-player/internal/theme"
)

// decorationPerBox accounts for the rounded border (1 col each side) plus
// Padding(0, 1) (1 col each side) that borderStyle applies to every box.
const decorationPerBox = 4

func borderStyle(t theme.Theme, focused bool) lipgloss.Style {
	color := t.Border
	if focused {
		color = t.Accent
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(0, 1)
}

func (m Model) renderNowPlaying(t theme.Theme) string {
	title := m.nowPlaying.title
	if title == "" {
		title = "(nothing playing — press enter on a channel)"
	} else if !m.nowPlaying.connected {
		title = "Reconnecting… (was: " + title + ")"
	}

	line1 := fmt.Sprintf("♪ %s", title)
	if m.nowPlaying.artist != "" {
		line1 += " — " + m.nowPlaying.artist
	}

	line2 := fmt.Sprintf("Channel: %s", m.nowPlaying.channel)
	if m.nowPlaying.bitrate > 0 {
		line2 += fmt.Sprintf("   •   %dk %s", m.nowPlaying.bitrate, m.nowPlaying.codec)
	}

	line3 := fmt.Sprintf("Elapsed: %s   Session: %s", m.nowPlaying.elapsed, m.session)
	line4 := m.renderVolumeBar()

	body := strings.Join([]string{line1, line2, line3, line4}, "\n")
	return borderStyle(t, m.focus == focusNowPlaying).Render(body)
}

func (m Model) renderVolumeBar() string {
	vol := clampVolume(m.cfg.Volume)
	filled := vol / 10
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)
	label := fmt.Sprintf("Vol: %s %d%%", bar, vol)
	if m.cfg.Muted {
		label += " (muted)"
	}
	return label
}

func (m Model) channelsHeader() string {
	label := "All"
	if m.channelsFilter == filterBookmarked {
		label = "Bookmarked"
	}
	return fmt.Sprintf("Channels ▸ [%s]  (a) all/bookmarked  (j/k) move", label)
}

func (m Model) channelsLines() []string {
	if m.channelsFilter == filterAll {
		lines := make([]string, len(m.channels))
		for i, ch := range m.channels {
			mark := "  "
			if bookmarks.IsChannelBookmarked(m.cfg, ch.Title) {
				mark = "★ "
			}
			lines[i] = fmt.Sprintf("%s%-24s %s", mark, ch.Title, ch.Genre)
		}
		return lines
	}
	lines := make([]string, len(m.cfg.BookmarkedChannels))
	copy(lines, m.cfg.BookmarkedChannels)
	return lines
}

func (m Model) tunesHeader() string {
	label := "History"
	if m.tunesMode == tunesBookmarked {
		label = "Bookmarked"
	}
	return fmt.Sprintf("Tunes ▸ [%s]  (H/s) history/bookmarked  (j/k) move", label)
}

func (m Model) tunesLines() []string {
	if m.tunesMode == tunesHistory {
		entries := m.hist.Entries()
		lines := make([]string, len(entries))
		for i, e := range entries {
			lines[i] = fmt.Sprintf("%s — %s (%s) @ %s", e.Title, e.Artist, e.Channel, e.PlayedAt.Format("15:04:05"))
		}
		return lines
	}
	lines := make([]string, len(m.cfg.BookmarkedTunes))
	for i, tu := range m.cfg.BookmarkedTunes {
		lines[i] = fmt.Sprintf("%s — %s (%s)", tu.Title, tu.Artist, tu.Channel)
	}
	return lines
}

func renderBox(t theme.Theme, focused bool, width int, header string, lines []string, selected int) string {
	rendered := make([]string, 0, len(lines)+1)
	rendered = append(rendered, header)
	if len(lines) == 0 {
		rendered = append(rendered, "(empty)")
	}
	for i, line := range lines {
		prefix := "  "
		if i == selected {
			prefix = "> "
		}
		rendered = append(rendered, prefix+line)
	}
	return borderStyle(t, focused).Width(width).Render(strings.Join(rendered, "\n"))
}

func (m Model) renderChannelsBox(t theme.Theme, width int) string {
	return renderBox(t, m.focus == focusChannels, width, m.channelsHeader(), m.channelsLines(), m.channelSelected)
}

func (m Model) renderTunesBox(t theme.Theme, width int) string {
	return renderBox(t, m.focus == focusTunes, width, m.tunesHeader(), m.tunesLines(), m.tuneSelected)
}

// boxWidth splits the terminal width evenly between the two side-by-side
// list boxes, minus each box's border/padding decoration. Falls back to
// defaultWidth before the first tea.WindowSizeMsg arrives.
func (m Model) boxWidth() int {
	w := m.width
	if w <= 0 {
		w = defaultWidth
	}
	usable := w - 2*decorationPerBox
	if usable < 2 {
		usable = 2
	}
	return usable / 2
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	t := theme.Get(m.cfg.Theme)

	width := m.boxWidth()
	lists := lipgloss.JoinHorizontal(lipgloss.Top, m.renderChannelsBox(t, width), m.renderTunesBox(t, width))

	footer := fmt.Sprintf("[Theme: %s]  tab focus · j/k move · enter play · b bookmark · a all/bookmarked · H/s tunes · +/- vol · m mute · t theme · r retry channels · q quit", t.Name)
	if m.errMsg != "" {
		footer = "Error: " + m.errMsg + "\n" + footer
	}

	return strings.Join([]string{
		m.renderNowPlaying(t),
		lists,
		footer,
	}, "\n")
}
```

- [ ] **Step 4: Run the full package test suite**

Run: `go test ./... -v`
Expected: all tests PASS across the whole module, including every test from Tasks 1–3.

- [ ] **Step 5: Format and build check**

Run: `gofmt -l internal/ui/view.go internal/ui/view_test.go && go build ./...`
Expected: `gofmt -l` prints nothing (no files need formatting); `go build ./...` exits 0.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/view.go internal/ui/view_test.go
git commit -m "feat: render Channels and Tunes as separate side-by-side boxes"
```

---

### Task 4: Manual visual verification (human, not agent)

**Files:** none — this task produces no diff.

Per this project's `CLAUDE.md`: interactive TUI behavior can only be verified by a human running the app in a real terminal, since sandboxed agent environments have no TTY.

- [ ] **Step 1: Run the app**

Run: `go run .`

- [ ] **Step 2: Verify checklist**

- Channels box and Tunes box render side by side under Now Playing, roughly evenly split.
- On first run with no bookmarked channels, Channels box defaults to `[All]`. Bookmark a channel with `b`, restart the app, and confirm it now defaults to `[Bookmarked]`.
- `a` toggles Channels between `[All]` and `[Bookmarked]`, resetting the selection to the top.
- `H` and `s` switch the Tunes box between `[History]` and `[Bookmarked]`.
- `Tab` cycles focus Now Playing → Channels → Tunes → Now Playing, with the focused box's border highlighted.
- `j`/`k` move the selection only within whichever box currently has focus; they do nothing while Now Playing is focused.
- Resize the terminal window and confirm both boxes resize to stay evenly split (validates the `tea.WindowSizeMsg` wiring).
- Footer text is legible and no longer mentions the removed `c`/`f` keys.

- [ ] **Step 3: Report back**

Note any visual issues (e.g. `decorationPerBox` under/over-estimating actual border+padding width) so the constant in `internal/ui/view.go` can be tuned in a follow-up commit.

---

## Plan Self-Review

**Spec coverage:**
- Channels box default (bookmarked if any exist, else all) + `a` toggle → Task 1 (`New()`, `toggleChannelsFilter`) + Task 3 (`channelsHeader`).
- Tunes box default History, secondary Bookmarked via `H`/`s` → Task 1 (`setTunesMode`) + Task 3 (`tunesHeader`).
- Two always-visible boxes, side by side → Task 3 (`View()`, `lipgloss.JoinHorizontal`).
- `Tab` cycles all three focus areas → Task 1.
- `j`/`k` discoverability (footer reorder + per-box header hint) → Task 3.
- Removal of `c`/`f` keys → Task 1 (`Update()` no longer has those cases).
- `handleBookmarkKey()` updated for new focus/filter/mode → Task 2.
- `WindowSizeMsg` handling + width-based box sizing → Task 1 (state) + Task 3 (`boxWidth`).
- Testing section (unit tests for state/filter logic and bookmark branching; manual verification for visuals) → Tasks 1–3 unit tests + Task 4.

**Placeholder scan:** no TBDs/TODOs or incomplete code blocks present.

**Type consistency:** `channelsFilter`/`tunesMode`/`focusArea` values and `Model` field names (`channelSelected`, `tuneSelected`, `width`) are used identically across all three tasks' code blocks. `defaultWidth` (Task 1) and `decorationPerBox` (Task 3) are the only two constants introduced, each defined once and referenced consistently.
