# SomaFM TUI Player Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go TUI (Bubble Tea) that plays SomaFM Icecast streams, with channel switching, dual bookmarks (channels + tunes), session history, volume/mute, and six cyclable color themes.

**Architecture:** Small, independently-testable packages (`config`, `theme`, `channels`, `history`, `bookmarks`, `player`) feed a single Bubble Tea root `Model` in `internal/ui` that owns all view state and rendering. The audio pipeline (shoutcast → go-mp3 → oto) runs behind a `player.Player` interface so the UI layer can be tested against a fake implementation without real audio or network I/O.

**Tech Stack:** Go 1.22+, `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`, `github.com/romantomjak/shoutcast`, `github.com/hajimehoshi/go-mp3`, `github.com/hajimehoshi/oto`.

## Global Constraints

- Module path: `github.com/jonasbn/somafm-player`.
- Always select the highest-quality **MP3** variant per channel — no AAC, no user-facing quality picker (spec: "Stream Variant Selection").
- Config file at `~/.config/somafm-player/config.json`, respecting `$XDG_CONFIG_HOME`; saved on every mutating action.
- History is in-memory only (ring buffer, max 5 entries), never persisted.
- Reconnection backoff schedule: 1s, 2s, 5s, 5s, 5s... (retry indefinitely, no cap).
- Theme cycle order is fixed: Nord → Dracula → Gruvbox → Tokyo Night → Solarized Dark → Solarized Light → Nord.
- Keybindings are fixed for v1, not user-remappable (spec: "Out of Scope").

---

## File Structure

```
somafm-player/
  go.mod
  main.go
  internal/
    config/
      config.go
      config_test.go
    theme/
      theme.go
      theme_test.go
    channels/
      channels.go
      channels_test.go
    history/
      history.go
      history_test.go
    bookmarks/
      bookmarks.go
      bookmarks_test.go
    player/
      player.go        // interface + shared message types
      fake.go           // FakePlayer for UI-layer tests
      fake_test.go
      volume.go         // volumeReader (PCM gain scaling)
      volume_test.go
      real.go           // shoutcast/go-mp3/oto wiring + reconnect backoff
    ui/
      model.go          // root Model: state, Init/Update, navigation & panel switching
      model_test.go
      playback.go        // channel-switch/track-change/reconnect handling
      playback_test.go
      timers.go          // elapsed/session tick handling + formatDuration
      timers_test.go
      volume.go          // volume/mute key handling
      volume_test.go
      bookmark_actions.go // context-sensitive `b` key handling
      bookmark_actions_test.go
      theme_actions.go   // `t` key handling
      view.go            // Now Playing + list panel rendering
```

---

### Task 1: Project scaffolding

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `internal/ui/model.go`
- Test: `internal/ui/model_test.go`

**Interfaces:**
- Produces: `ui.Model` (satisfies `tea.Model`: `Init() tea.Cmd`, `Update(tea.Msg) (tea.Model, tea.Cmd)`, `View() string`), constructor `ui.New() Model`.

- [ ] **Step 1: Initialize the module and add UI dependencies**

```bash
go mod init github.com/jonasbn/somafm-player
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
```

- [ ] **Step 2: Write the failing test for quit behavior**

`internal/ui/model_test.go`:
```go
package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdate_QuitsOnQ(t *testing.T) {
	m := New()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	if cmd == nil {
		t.Fatal("expected a command to be returned for quit key")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/ui/... -run TestUpdate_QuitsOnQ -v`
Expected: FAIL (`ui.New` undefined)

- [ ] **Step 4: Write minimal model implementation**

`internal/ui/model.go`:
```go
package ui

import tea "github.com/charmbracelet/bubbletea"

type Model struct {
	quitting bool
}

func New() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	return "somafm-player (press q to quit)\n"
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/ui/... -run TestUpdate_QuitsOnQ -v`
Expected: PASS

- [ ] **Step 6: Write main.go entrypoint**

`main.go`:
```go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jonasbn/somafm-player/internal/ui"
)

func main() {
	p := tea.NewProgram(ui.New())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 7: Build and manually verify it launches and quits**

Run: `go build ./... && go run .`
Expected: terminal shows "somafm-player (press q to quit)"; pressing `q` exits cleanly back to the shell prompt.

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum main.go internal/ui/model.go internal/ui/model_test.go
git commit -m "feat: scaffold Bubble Tea app with quit handling"
```

---

### Task 2: Config package

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `config.Config{LastChannel, Volume, Muted, Theme string/int/bool, BookmarkedChannels []string, BookmarkedTunes []config.BookmarkedTune}`, `config.BookmarkedTune{Title, Artist, Channel string; BookmarkedAt time.Time}`, `config.DefaultConfig() Config`, `config.Path() (string, error)`, `config.Load() (Config, error)`, `config.Save(cfg Config) error`.

- [ ] **Step 1: Write the failing round-trip test**

`internal/config/config_test.go`:
```go
package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveThenLoad_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := Config{
		LastChannel: "Drone Zone",
		Volume:      65,
		Muted:       false,
		Theme:       "Dracula",
		BookmarkedChannels: []string{"Drone Zone", "Groove Salad"},
		BookmarkedTunes: []BookmarkedTune{
			{Title: "Track", Artist: "Artist", Channel: "Drone Zone", BookmarkedAt: time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)},
		},
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.LastChannel != cfg.LastChannel || loaded.Volume != cfg.Volume || loaded.Theme != cfg.Theme {
		t.Fatalf("loaded config %+v does not match saved config %+v", loaded, cfg)
	}
	if len(loaded.BookmarkedChannels) != 2 || len(loaded.BookmarkedTunes) != 1 {
		t.Fatalf("loaded config lists did not round-trip: %+v", loaded)
	}

	wantPath := filepath.Join(dir, "somafm-player", "config.json")
	gotPath, err := Path()
	if err != nil {
		t.Fatalf("Path returned error: %v", err)
	}
	if gotPath != wantPath {
		t.Fatalf("Path() = %q, want %q", gotPath, wantPath)
	}
}

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	def := DefaultConfig()
	if cfg.Volume != def.Volume || cfg.Theme != def.Theme {
		t.Fatalf("Load() on missing file = %+v, want defaults %+v", cfg, def)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -v`
Expected: FAIL (package/functions undefined)

- [ ] **Step 3: Write the implementation**

`internal/config/config.go`:
```go
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type BookmarkedTune struct {
	Title        string    `json:"title"`
	Artist       string    `json:"artist"`
	Channel      string    `json:"channel"`
	BookmarkedAt time.Time `json:"bookmarkedAt"`
}

type Config struct {
	LastChannel        string           `json:"lastChannel"`
	Volume             int              `json:"volume"`
	Muted              bool             `json:"muted"`
	Theme              string           `json:"theme"`
	BookmarkedChannels []string         `json:"bookmarkedChannels"`
	BookmarkedTunes    []BookmarkedTune `json:"bookmarkedTunes"`
}

func DefaultConfig() Config {
	return Config{Volume: 80, Theme: "Nord"}
}

func Path() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "somafm-player", "config.json"), nil
}

func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add config load/save at XDG config path"
```

---

### Task 3: Theme package

**Files:**
- Create: `internal/theme/theme.go`
- Test: `internal/theme/theme_test.go`

**Interfaces:**
- Produces: `theme.Theme{Name string; Background, Foreground, Accent, Border, Muted lipgloss.Color}`, `theme.Order []string` (fixed cycle order), `theme.Get(name string) Theme`, `theme.Next(name string) string`.

- [ ] **Step 1: Write the failing test**

`internal/theme/theme_test.go`:
```go
package theme

import "testing"

func TestOrder_HasSixThemes(t *testing.T) {
	want := []string{"Nord", "Dracula", "Gruvbox", "Tokyo Night", "Solarized Dark", "Solarized Light"}
	if len(Order) != len(want) {
		t.Fatalf("Order has %d themes, want %d", len(Order), len(want))
	}
	for i, name := range want {
		if Order[i] != name {
			t.Fatalf("Order[%d] = %q, want %q", i, Order[i], name)
		}
	}
}

func TestNext_CyclesAndWrapsAround(t *testing.T) {
	if got := Next("Nord"); got != "Dracula" {
		t.Fatalf("Next(Nord) = %q, want Dracula", got)
	}
	if got := Next("Solarized Light"); got != "Nord" {
		t.Fatalf("Next(Solarized Light) = %q, want Nord (wrap around)", got)
	}
}

func TestGet_UnknownNameFallsBackToNord(t *testing.T) {
	got := Get("Not A Theme")
	if got.Name != "Nord" {
		t.Fatalf("Get(unknown).Name = %q, want Nord", got.Name)
	}
}

func TestGet_AllOrderNamesResolve(t *testing.T) {
	for _, name := range Order {
		if got := Get(name); got.Name != name {
			t.Fatalf("Get(%q).Name = %q, want %q", name, got.Name, name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/theme/... -v`
Expected: FAIL (package undefined)

- [ ] **Step 3: Write the implementation**

`internal/theme/theme.go`:
```go
package theme

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Name       string
	Background lipgloss.Color
	Foreground lipgloss.Color
	Accent     lipgloss.Color
	Border     lipgloss.Color
	Muted      lipgloss.Color
}

var Order = []string{
	"Nord",
	"Dracula",
	"Gruvbox",
	"Tokyo Night",
	"Solarized Dark",
	"Solarized Light",
}

var themes = map[string]Theme{
	"Nord":             {Name: "Nord", Background: "#2E3440", Foreground: "#D8DEE9", Accent: "#88C0D0", Border: "#4C566A", Muted: "#4C566A"},
	"Dracula":          {Name: "Dracula", Background: "#282A36", Foreground: "#F8F8F2", Accent: "#BD93F9", Border: "#44475A", Muted: "#6272A4"},
	"Gruvbox":          {Name: "Gruvbox", Background: "#282828", Foreground: "#EBDBB2", Accent: "#FE8019", Border: "#504945", Muted: "#928374"},
	"Tokyo Night":      {Name: "Tokyo Night", Background: "#1A1B26", Foreground: "#C0CAF5", Accent: "#7AA2F7", Border: "#3B4261", Muted: "#565F89"},
	"Solarized Dark":   {Name: "Solarized Dark", Background: "#002B36", Foreground: "#839496", Accent: "#268BD2", Border: "#073642", Muted: "#586E75"},
	"Solarized Light":  {Name: "Solarized Light", Background: "#FDF6E3", Foreground: "#657B83", Accent: "#268BD2", Border: "#EEE8D5", Muted: "#93A1A1"},
}

func Get(name string) Theme {
	if t, ok := themes[name]; ok {
		return t
	}
	return themes["Nord"]
}

func Next(name string) string {
	for i, n := range Order {
		if n == name {
			return Order[(i+1)%len(Order)]
		}
	}
	return Order[0]
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/theme/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/theme/theme.go internal/theme/theme_test.go
git commit -m "feat: add six-theme palette and cycling"
```

---

### Task 4: Channels package

**Files:**
- Create: `internal/channels/channels.go`
- Test: `internal/channels/channels_test.go`

**Interfaces:**
- Produces: `channels.Playlist{URL, Format, Quality string}`, `channels.Channel{ID, Title, Description, Genre string; Playlists []Playlist}`, `channels.Parse(data []byte) ([]Channel, error)`, `channels.Fetch(ctx context.Context, url string) ([]Channel, error)`, `(c Channel) BestMP3Stream() string`, `channels.ResolveStreamURL(ctx context.Context, plsURL string) (string, error)`, `channels.ParseBitrateFromURL(url string) (bitrate int, codec string)`.

- [ ] **Step 1: Write the failing tests**

`internal/channels/channels_test.go`:
```go
package channels

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const sampleChannelsJSON = `{
  "channels": [
    {
      "id": "dronezone",
      "title": "Drone Zone",
      "description": "Served best chilled, safe with most food groups.",
      "genre": "ambient",
      "playlists": [
        {"url": "http://example.test/dronezone.pls", "format": "mp3", "quality": "highest"},
        {"url": "http://example.test/dronezonelo.pls", "format": "mp3", "quality": "low"},
        {"url": "http://example.test/dronezone-aac.pls", "format": "aac", "quality": "highest"}
      ]
    }
  ]
}`

const samplePLS = "[playlist]\nNumberOfEntries=1\nFile1=https://ice5.somafm.com/dronezone-128-mp3\nTitle1=SomaFM: Drone Zone\nLength1=-1\nVersion=2\n"

func TestParse_ExtractsChannels(t *testing.T) {
	chs, err := Parse([]byte(sampleChannelsJSON))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(chs) != 1 {
		t.Fatalf("Parse returned %d channels, want 1", len(chs))
	}
	if chs[0].Title != "Drone Zone" || chs[0].Genre != "ambient" {
		t.Fatalf("unexpected channel: %+v", chs[0])
	}
}

func TestBestMP3Stream_PicksHighestQualityMP3(t *testing.T) {
	chs, err := Parse([]byte(sampleChannelsJSON))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	got := chs[0].BestMP3Stream()
	want := "http://example.test/dronezone.pls"
	if got != want {
		t.Fatalf("BestMP3Stream() = %q, want %q", got, want)
	}
}

func TestFetch_ParsesFromHTTPServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(sampleChannelsJSON))
	}))
	defer srv.Close()

	chs, err := Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(chs) != 1 || chs[0].Title != "Drone Zone" {
		t.Fatalf("unexpected fetch result: %+v", chs)
	}
}

func TestResolveStreamURL_ParsesFile1FromPLS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(samplePLS))
	}))
	defer srv.Close()

	got, err := ResolveStreamURL(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("ResolveStreamURL returned error: %v", err)
	}
	want := "https://ice5.somafm.com/dronezone-128-mp3"
	if got != want {
		t.Fatalf("ResolveStreamURL() = %q, want %q", got, want)
	}
}

func TestParseBitrateFromURL_ExtractsBitrateAndCodec(t *testing.T) {
	bitrate, codec := ParseBitrateFromURL("https://ice5.somafm.com/dronezone-128-mp3")
	if bitrate != 128 || codec != "MP3" {
		t.Fatalf("ParseBitrateFromURL() = (%d, %q), want (128, \"MP3\")", bitrate, codec)
	}
}

func TestParseBitrateFromURL_NoMatchReturnsZeroValue(t *testing.T) {
	bitrate, codec := ParseBitrateFromURL("https://example.test/not-a-stream-url")
	if bitrate != 0 || codec != "" {
		t.Fatalf("ParseBitrateFromURL() = (%d, %q), want (0, \"\")", bitrate, codec)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/channels/... -v`
Expected: FAIL (package undefined)

- [ ] **Step 3: Write the implementation**

`internal/channels/channels.go`:
```go
package channels

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type Playlist struct {
	URL     string `json:"url"`
	Format  string `json:"format"`
	Quality string `json:"quality"`
}

type Channel struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Genre       string     `json:"genre"`
	Playlists   []Playlist `json:"playlists"`
}

type channelsResponse struct {
	Channels []Channel `json:"channels"`
}

func Parse(data []byte) ([]Channel, error) {
	var resp channelsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Channels, nil
}

func Fetch(ctx context.Context, url string) ([]Channel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

func qualityRank(q string) int {
	switch q {
	case "highest":
		return 3
	case "high":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func (c Channel) BestMP3Stream() string {
	var best Playlist
	found := false
	for _, p := range c.Playlists {
		if p.Format != "mp3" {
			continue
		}
		if !found || qualityRank(p.Quality) > qualityRank(best.Quality) {
			best = p
			found = true
		}
	}
	if !found {
		return ""
	}
	return best.URL
}

func ResolveStreamURL(ctx context.Context, plsURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, plsURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "File1=") {
			return strings.TrimPrefix(line, "File1="), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no File1 entry found in %s", plsURL)
}

var bitratePattern = regexp.MustCompile(`-(\d+)-(mp3|aac)`)

func ParseBitrateFromURL(url string) (bitrate int, codec string) {
	m := bitratePattern.FindStringSubmatch(url)
	if m == nil {
		return 0, ""
	}
	bitrate, _ = strconv.Atoi(m[1])
	return bitrate, strings.ToUpper(m[2])
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/channels/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/channels/channels.go internal/channels/channels_test.go
git commit -m "feat: add channels.json fetch/parse, .pls resolution, bitrate parsing"
```

---

### Task 5: History package

**Files:**
- Create: `internal/history/history.go`
- Test: `internal/history/history_test.go`

**Interfaces:**
- Produces: `history.Entry{Title, Artist, Channel string; PlayedAt time.Time}`, `history.New(max int) *History`, `(h *History) Add(e Entry)`, `(h *History) Entries() []Entry` (most recent first).

- [ ] **Step 1: Write the failing test**

`internal/history/history_test.go`:
```go
package history

import (
	"testing"
	"time"
)

func TestAdd_MostRecentFirst(t *testing.T) {
	h := New(5)
	h.Add(Entry{Title: "First", PlayedAt: time.Unix(1, 0)})
	h.Add(Entry{Title: "Second", PlayedAt: time.Unix(2, 0)})

	entries := h.Entries()
	if len(entries) != 2 {
		t.Fatalf("Entries() has %d items, want 2", len(entries))
	}
	if entries[0].Title != "Second" || entries[1].Title != "First" {
		t.Fatalf("Entries() = %+v, want [Second, First]", entries)
	}
}

func TestAdd_CapsAtMax(t *testing.T) {
	h := New(5)
	for i := 0; i < 8; i++ {
		h.Add(Entry{Title: string(rune('A' + i)), PlayedAt: time.Unix(int64(i), 0)})
	}
	entries := h.Entries()
	if len(entries) != 5 {
		t.Fatalf("Entries() has %d items, want capped at 5", len(entries))
	}
	if entries[0].Title != "H" {
		t.Fatalf("Entries()[0].Title = %q, want %q (most recent)", entries[0].Title, "H")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/history/... -v`
Expected: FAIL (package undefined)

- [ ] **Step 3: Write the implementation**

`internal/history/history.go`:
```go
package history

import "time"

type Entry struct {
	Title    string
	Artist   string
	Channel  string
	PlayedAt time.Time
}

type History struct {
	entries []Entry
	max     int
}

func New(max int) *History {
	return &History{max: max}
}

func (h *History) Add(e Entry) {
	h.entries = append([]Entry{e}, h.entries...)
	if len(h.entries) > h.max {
		h.entries = h.entries[:h.max]
	}
}

func (h *History) Entries() []Entry {
	return h.entries
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/history/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/history/history.go internal/history/history_test.go
git commit -m "feat: add session-only track history ring buffer"
```

---

### Task 6: Bookmarks package

**Files:**
- Create: `internal/bookmarks/bookmarks.go`
- Test: `internal/bookmarks/bookmarks_test.go`

**Interfaces:**
- Consumes: `config.Config`, `config.BookmarkedTune` (Task 2).
- Produces: `bookmarks.ToggleChannel(cfg *config.Config, channelTitle string)`, `bookmarks.IsChannelBookmarked(cfg config.Config, channelTitle string) bool`, `bookmarks.AddTune(cfg *config.Config, tune config.BookmarkedTune)` (deduped by Title+Artist+Channel).

- [ ] **Step 1: Write the failing tests**

`internal/bookmarks/bookmarks_test.go`:
```go
package bookmarks

import (
	"testing"

	"github.com/jonasbn/somafm-player/internal/config"
)

func TestToggleChannel_AddsThenRemoves(t *testing.T) {
	cfg := config.Config{}

	ToggleChannel(&cfg, "Drone Zone")
	if !IsChannelBookmarked(cfg, "Drone Zone") {
		t.Fatal("expected Drone Zone to be bookmarked after first toggle")
	}

	ToggleChannel(&cfg, "Drone Zone")
	if IsChannelBookmarked(cfg, "Drone Zone") {
		t.Fatal("expected Drone Zone to be unbookmarked after second toggle")
	}
}

func TestAddTune_DedupesBySameTitleArtistChannel(t *testing.T) {
	cfg := config.Config{}
	tune := config.BookmarkedTune{Title: "Track", Artist: "Artist", Channel: "Drone Zone"}

	AddTune(&cfg, tune)
	AddTune(&cfg, tune)

	if len(cfg.BookmarkedTunes) != 1 {
		t.Fatalf("BookmarkedTunes has %d entries, want 1 (deduped)", len(cfg.BookmarkedTunes))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/bookmarks/... -v`
Expected: FAIL (package undefined)

- [ ] **Step 3: Write the implementation**

`internal/bookmarks/bookmarks.go`:
```go
package bookmarks

import "github.com/jonasbn/somafm-player/internal/config"

func ToggleChannel(cfg *config.Config, channelTitle string) {
	for i, c := range cfg.BookmarkedChannels {
		if c == channelTitle {
			cfg.BookmarkedChannels = append(cfg.BookmarkedChannels[:i], cfg.BookmarkedChannels[i+1:]...)
			return
		}
	}
	cfg.BookmarkedChannels = append(cfg.BookmarkedChannels, channelTitle)
}

func IsChannelBookmarked(cfg config.Config, channelTitle string) bool {
	for _, c := range cfg.BookmarkedChannels {
		if c == channelTitle {
			return true
		}
	}
	return false
}

func AddTune(cfg *config.Config, tune config.BookmarkedTune) {
	for _, t := range cfg.BookmarkedTunes {
		if t.Title == tune.Title && t.Artist == tune.Artist && t.Channel == tune.Channel {
			return
		}
	}
	cfg.BookmarkedTunes = append(cfg.BookmarkedTunes, tune)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/bookmarks/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/bookmarks/bookmarks.go internal/bookmarks/bookmarks_test.go
git commit -m "feat: add channel bookmark toggle and deduped tune bookmarking"
```

---

### Task 7: Player interface, fake player, and volume scaling

**Files:**
- Create: `internal/player/player.go`
- Create: `internal/player/fake.go`
- Create: `internal/player/volume.go`
- Test: `internal/player/fake_test.go`
- Test: `internal/player/volume_test.go`

**Interfaces:**
- Produces: `player.Msg interface{}`, `player.TrackChangedMsg{Title, Artist string}`, `player.ConnectionLostMsg{}`, `player.ReconnectedMsg{}`, `player.Player interface{ Play(streamURL string); Stop(); SetVolume(percent int); SetMuted(muted bool); Messages() <-chan Msg }`, `player.NewFakePlayer() *FakePlayer` with test helpers `(*FakePlayer) Emit(msg Msg)`, `(*FakePlayer) PlayedURLs() []string`, `(*FakePlayer) Volume() int`, `(*FakePlayer) Muted() bool`, `(*FakePlayer) Stopped() bool`.

- [ ] **Step 1: Write the failing fake-player test**

`internal/player/fake_test.go`:
```go
package player

import "testing"

func TestFakePlayer_TracksPlayCallsAndSettings(t *testing.T) {
	fp := NewFakePlayer()

	fp.Play("https://example.test/stream-128-mp3")
	fp.SetVolume(42)
	fp.SetMuted(true)

	if got := fp.PlayedURLs(); len(got) != 1 || got[0] != "https://example.test/stream-128-mp3" {
		t.Fatalf("PlayedURLs() = %v, want one entry for the played URL", got)
	}
	if fp.Volume() != 42 {
		t.Fatalf("Volume() = %d, want 42", fp.Volume())
	}
	if !fp.Muted() {
		t.Fatal("Muted() = false, want true")
	}

	fp.Stop()
	if !fp.Stopped() {
		t.Fatal("Stopped() = false after Stop() was called")
	}
}

func TestFakePlayer_EmitDeliversOnMessagesChannel(t *testing.T) {
	fp := NewFakePlayer()

	fp.Emit(TrackChangedMsg{Title: "Song", Artist: "Band"})

	msg := <-fp.Messages()
	tc, ok := msg.(TrackChangedMsg)
	if !ok || tc.Title != "Song" || tc.Artist != "Band" {
		t.Fatalf("Messages() delivered %+v, want TrackChangedMsg{Song, Band}", msg)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/player/... -run FakePlayer -v`
Expected: FAIL (package undefined)

- [ ] **Step 3: Write player.go and fake.go**

`internal/player/player.go`:
```go
package player

type Msg interface{}

type TrackChangedMsg struct {
	Title  string
	Artist string
}

type ConnectionLostMsg struct{}

type ReconnectedMsg struct{}

type Player interface {
	Play(streamURL string)
	Stop()
	SetVolume(percent int)
	SetMuted(muted bool)
	Messages() <-chan Msg
}
```

`internal/player/fake.go`:
```go
package player

import "sync"

type FakePlayer struct {
	mu         sync.Mutex
	msgs       chan Msg
	playedURLs []string
	volume     int
	muted      bool
	stopped    bool
}

func NewFakePlayer() *FakePlayer {
	return &FakePlayer{msgs: make(chan Msg, 16), volume: 80}
}

func (p *FakePlayer) Play(streamURL string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.playedURLs = append(p.playedURLs, streamURL)
	p.stopped = false
}

func (p *FakePlayer) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopped = true
}

func (p *FakePlayer) SetVolume(percent int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.volume = percent
}

func (p *FakePlayer) SetMuted(muted bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.muted = muted
}

func (p *FakePlayer) Messages() <-chan Msg {
	return p.msgs
}

func (p *FakePlayer) Emit(msg Msg) {
	p.msgs <- msg
}

func (p *FakePlayer) PlayedURLs() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.playedURLs...)
}

func (p *FakePlayer) Volume() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volume
}

func (p *FakePlayer) Muted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.muted
}

func (p *FakePlayer) Stopped() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stopped
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/player/... -run FakePlayer -v`
Expected: PASS

- [ ] **Step 5: Write the failing volume-scaling test**

`internal/player/volume_test.go`:
```go
package player

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

func samplesToBytes(samples []int16) []byte {
	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	return buf
}

func TestVolumeReader_ScalesSamplesByFactor(t *testing.T) {
	src := bytes.NewReader(samplesToBytes([]int16{1000, -1000, 2000}))
	vr := newVolumeReader(src, func() float64 { return 0.5 })

	out := make([]byte, 64)
	n, err := vr.Read(out)
	if err != nil && err != io.EOF {
		t.Fatalf("Read returned error: %v", err)
	}

	got := make([]int16, n/2)
	for i := range got {
		got[i] = int16(binary.LittleEndian.Uint16(out[i*2 : i*2+2]))
	}
	want := []int16{500, -500, 1000}
	if len(got) != len(want) {
		t.Fatalf("Read() produced %d samples, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestVolumeReader_ZeroFactorSilences(t *testing.T) {
	src := bytes.NewReader(samplesToBytes([]int16{12345, -12345}))
	vr := newVolumeReader(src, func() float64 { return 0 })

	out := make([]byte, 64)
	n, _ := vr.Read(out)

	for i := 0; i < n; i++ {
		if out[i] != 0 {
			t.Fatalf("byte[%d] = %d, want 0 when muted", i, out[i])
		}
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/player/... -run VolumeReader -v`
Expected: FAIL (`newVolumeReader` undefined)

- [ ] **Step 7: Write volume.go**

`internal/player/volume.go`:
```go
package player

import (
	"encoding/binary"
	"io"
)

type volumeReader struct {
	src    io.Reader
	factor func() float64
}

func newVolumeReader(src io.Reader, factor func() float64) *volumeReader {
	return &volumeReader{src: src, factor: factor}
}

func (r *volumeReader) Read(p []byte) (int, error) {
	n, err := r.src.Read(p)
	if n == 0 {
		return n, err
	}
	v := r.factor()
	usable := n - (n % 2)
	for i := 0; i < usable; i += 2 {
		sample := int16(binary.LittleEndian.Uint16(p[i : i+2]))
		scaled := int16(float64(sample) * v)
		binary.LittleEndian.PutUint16(p[i:i+2], uint16(scaled))
	}
	return n, err
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/player/... -run VolumeReader -v`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/player/player.go internal/player/fake.go internal/player/volume.go internal/player/fake_test.go internal/player/volume_test.go
git commit -m "feat: add player interface, fake player, and PCM volume scaling"
```

---

### Task 8: Real player (shoutcast → go-mp3 → oto, with reconnect backoff)

**Files:**
- Create: `internal/player/real.go`

**Interfaces:**
- Consumes: `player.Msg`, `player.TrackChangedMsg`, `player.ConnectionLostMsg`, `player.ReconnectedMsg`, `newVolumeReader` (Task 7).
- Produces: `player.NewRealPlayer() *RealPlayer` (satisfies `player.Player`).

This task wires three external libraries whose exact method signatures should be confirmed against the versions actually resolved by `go get`, since minor API differences across versions are common for audio libraries.

- [ ] **Step 1: Add the audio dependencies and inspect their APIs**

```bash
go get github.com/romantomjak/shoutcast@latest
go get github.com/hajimehoshi/go-mp3@latest
go get github.com/hajimehoshi/oto@latest
go doc github.com/romantomjak/shoutcast
go doc github.com/hajimehoshi/go-mp3
go doc github.com/hajimehoshi/oto
```

Read the output of the three `go doc` calls. Confirm (adjusting the code in Step 2 if needed):
- The shoutcast open function's exact name/signature and how it exposes a metadata callback and an `io.Reader` for cleaned audio.
- `mp3.NewDecoder`'s signature and how it exposes `SampleRate()`.
- `oto.NewContext`'s signature and how to obtain an `io.Writer`-like player to write PCM bytes to.

- [ ] **Step 2: Write real.go**

`internal/player/real.go`:
```go
package player

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/go-mp3"
	"github.com/hajimehoshi/oto"
	"github.com/romantomjak/shoutcast"
)

var backoffSchedule = []time.Duration{time.Second, 2 * time.Second, 5 * time.Second}

type RealPlayer struct {
	msgs   chan Msg
	mu     sync.Mutex
	volume int
	muted  bool
	cancel context.CancelFunc
}

func NewRealPlayer() *RealPlayer {
	return &RealPlayer{msgs: make(chan Msg, 16), volume: 80}
}

func (p *RealPlayer) Messages() <-chan Msg { return p.msgs }

func (p *RealPlayer) SetVolume(percent int) {
	p.mu.Lock()
	p.volume = percent
	p.mu.Unlock()
}

func (p *RealPlayer) SetMuted(muted bool) {
	p.mu.Lock()
	p.muted = muted
	p.mu.Unlock()
}

func (p *RealPlayer) volumeFactor() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.muted {
		return 0
	}
	return float64(p.volume) / 100.0
}

func (p *RealPlayer) Play(streamURL string) {
	if p.cancel != nil {
		p.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go p.run(ctx, streamURL)
}

func (p *RealPlayer) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *RealPlayer) run(ctx context.Context, streamURL string) {
	attempt := 0
	for {
		if ctx.Err() != nil {
			return
		}
		err := p.playOnce(ctx, streamURL)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			p.msgs <- ConnectionLostMsg{}
			delay := backoffSchedule[len(backoffSchedule)-1]
			if attempt < len(backoffSchedule) {
				delay = backoffSchedule[attempt]
			}
			attempt++
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
			continue
		}
		attempt = 0
	}
}

func (p *RealPlayer) playOnce(ctx context.Context, streamURL string) error {
	stream, err := shoutcast.Open(streamURL)
	if err != nil {
		return err
	}
	defer stream.Close()

	stream.MetadataCallbackFunc(func(m shoutcast.Metadata) {
		title, artist := splitStreamTitle(m.StreamTitle())
		p.msgs <- TrackChangedMsg{Title: title, Artist: artist}
	})

	decoder, err := mp3.NewDecoder(stream)
	if err != nil {
		return err
	}

	otoCtx, err := oto.NewContext(decoder.SampleRate(), 2, 2)
	if err != nil {
		return err
	}
	defer otoCtx.Close()

	otoPlayer := otoCtx.NewPlayer()
	defer otoPlayer.Close()

	vr := newVolumeReader(decoder, p.volumeFactor)
	p.msgs <- ReconnectedMsg{}

	buf := make([]byte, 4096)
	for {
		if ctx.Err() != nil {
			return nil
		}
		n, readErr := vr.Read(buf)
		if n > 0 {
			if _, writeErr := otoPlayer.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
		}
		if readErr != nil {
			return readErr
		}
	}
}

func splitStreamTitle(raw string) (title, artist string) {
	parts := strings.SplitN(raw, " - ", 2)
	if len(parts) == 2 {
		return parts[1], parts[0]
	}
	return raw, ""
}
```

- [ ] **Step 3: Build to verify it compiles against the resolved library versions**

Run: `go build ./...`
Expected: builds cleanly. If any of the three library calls don't match the signatures found in Step 1's `go doc` output, adjust `real.go` accordingly and rebuild until it compiles.

- [ ] **Step 4: Manual smoke test against a real stream**

Add a temporary `main.go` snippet (or a `//go:build manual` test file) that resolves `https://somafm.com/dronezone.pls` via `channels.ResolveStreamURL` and calls `NewRealPlayer().Play(url)`, then sleeps 10s. Run it and confirm audio is audible and a `TrackChangedMsg` prints. Delete the temporary snippet afterward.

- [ ] **Step 5: Commit**

```bash
git add internal/player/real.go go.mod go.sum
git commit -m "feat: wire real audio pipeline (shoutcast, go-mp3, oto) with reconnect backoff"
```

---

### Task 9: Root UI model — state, navigation, and panel switching

**Files:**
- Modify: `internal/ui/model.go`
- Test: `internal/ui/model_test.go`

**Interfaces:**
- Consumes: `config.Config` (Task 2), `channels.Channel` (Task 4), `player.Player` (Task 7).
- Produces: `ui.New(cfg config.Config, chs []channels.Channel, p player.Player, hist *history.History) Model`; internal `viewMode` (`viewChannels`, `viewBookmarkedChannels`, `viewBookmarkedTunes`, `viewHistory`) and `focusArea` (`focusNowPlaying`, `focusList`) enums, used by later tasks.

- [ ] **Step 1: Write the failing navigation tests**

`internal/ui/model_test.go` (replace the file's contents, keeping the quit test):
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

func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestUpdate_QuitsOnQ(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(key("q"))
	if cmd == nil || cmd().(tea.QuitMsg) != (tea.QuitMsg{}) {
		t.Fatal("expected q to produce tea.Quit")
	}
}

func TestUpdate_JKMovesSelectionWithinBounds(t *testing.T) {
	m := newTestModel()

	next, _ := m.Update(key("j"))
	m = next.(Model)
	if m.selected != 1 {
		t.Fatalf("selected = %d after j, want 1", m.selected)
	}

	next, _ = m.Update(key("j")) // already at last item, should not overflow
	m = next.(Model)
	if m.selected != 1 {
		t.Fatalf("selected = %d after second j at bottom, want clamped at 1", m.selected)
	}

	next, _ = m.Update(key("k"))
	m = next.(Model)
	if m.selected != 0 {
		t.Fatalf("selected = %d after k, want 0", m.selected)
	}
}

func TestUpdate_PanelSwitchKeysChangeModeAndResetSelection(t *testing.T) {
	m := newTestModel()
	m.selected = 1

	next, _ := m.Update(key("f"))
	m = next.(Model)
	if m.mode != viewBookmarkedChannels || m.selected != 0 {
		t.Fatalf("after f: mode=%v selected=%d, want viewBookmarkedChannels/0", m.mode, m.selected)
	}

	next, _ = m.Update(key("s"))
	m = next.(Model)
	if m.mode != viewBookmarkedTunes {
		t.Fatalf("after s: mode=%v, want viewBookmarkedTunes", m.mode)
	}

	next, _ = m.Update(key("H"))
	m = next.(Model)
	if m.mode != viewHistory {
		t.Fatalf("after H: mode=%v, want viewHistory", m.mode)
	}

	next, _ = m.Update(key("c"))
	m = next.(Model)
	if m.mode != viewChannels {
		t.Fatalf("after c: mode=%v, want viewChannels", m.mode)
	}
}

func TestUpdate_TabTogglesFocus(t *testing.T) {
	m := newTestModel()
	if m.focus != focusNowPlaying {
		t.Fatalf("initial focus = %v, want focusNowPlaying", m.focus)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.focus != focusList {
		t.Fatalf("focus after tab = %v, want focusList", m.focus)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.focus != focusNowPlaying {
		t.Fatalf("focus after second tab = %v, want focusNowPlaying", m.focus)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/... -v`
Expected: FAIL (`New` has wrong signature, `viewMode`/`focusArea` undefined)

- [ ] **Step 3: Rewrite model.go with full state and navigation handling**

`internal/ui/model.go`:
```go
package ui

import (
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
	title     string
	artist    string
	channel   string
	bitrate   int
	codec     string
	connected bool
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
}

func New(cfg config.Config, chs []channels.Channel, p player.Player, hist *history.History) Model {
	return Model{
		cfg:      cfg,
		channels: chs,
		player:   p,
		hist:     hist,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
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
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	return "somafm-player\n"
}
```

- [ ] **Step 4: Update main.go for the new constructor**

`main.go`:
```go
package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jonasbn/somafm-player/internal/channels"
	"github.com/jonasbn/somafm-player/internal/config"
	"github.com/jonasbn/somafm-player/internal/history"
	"github.com/jonasbn/somafm-player/internal/player"
	"github.com/jonasbn/somafm-player/internal/ui"
)

const channelsURL = "https://somafm.com/channels.json"

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading config:", err)
		os.Exit(1)
	}

	chs, err := channels.Fetch(context.Background(), channelsURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error fetching channel list:", err)
		os.Exit(1)
	}

	m := ui.New(cfg, chs, player.NewRealPlayer(), history.New(5))
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/ui/... -v && go build ./...`
Expected: PASS, clean build

- [ ] **Step 6: Commit**

```bash
git add internal/ui/model.go internal/ui/model_test.go main.go
git commit -m "feat: add root UI model with navigation, focus, and panel switching"
```

---

### Task 10: Player integration — play on enter, track/connection messages, history recording

**Files:**
- Create: `internal/ui/playback.go`
- Modify: `internal/ui/model.go`
- Test: `internal/ui/playback_test.go`

**Interfaces:**
- Consumes: `channels.ResolveStreamURL`, `channels.ParseBitrateFromURL`, `Channel.BestMP3Stream` (Task 4); `player.TrackChangedMsg`, `player.ConnectionLostMsg`, `player.ReconnectedMsg` (Task 7); `history.Entry` (Task 5).
- Produces: `streamResolvedMsg{channelTitle, streamURL string; err error}`, `resolveAndPlayCmd(ch channels.Channel) tea.Cmd`, `waitForPlayerMsg(p player.Player) tea.Cmd`, `(m Model) recordCurrentTrackToHistory() Model`. `Model.Init` now starts the player message pump.

- [ ] **Step 1: Write the failing tests**

`internal/ui/playback_test.go`:
```go
package ui

import (
	"testing"

	"github.com/jonasbn/somafm-player/internal/channels"
	"github.com/jonasbn/somafm-player/internal/config"
	"github.com/jonasbn/somafm-player/internal/history"
	"github.com/jonasbn/somafm-player/internal/player"
)

func TestUpdate_EnterOnChannelResolvesAndPlays(t *testing.T) {
	m := newTestModel()

	_, cmd := m.Update(key("enter"))
	if cmd == nil {
		t.Fatal("expected enter to return a resolve command")
	}
	msg := cmd()
	resolved, ok := msg.(streamResolvedMsg)
	if !ok {
		t.Fatalf("expected streamResolvedMsg, got %T", msg)
	}
	if resolved.channelTitle != "Groove Salad" {
		t.Fatalf("resolved.channelTitle = %q, want Groove Salad", resolved.channelTitle)
	}
}

func TestUpdate_StreamResolvedMsgStartsPlaybackAndSetsNowPlaying(t *testing.T) {
	fp := player.NewFakePlayer()
	m := New(config.DefaultConfig(), []channels.Channel{{Title: "Drone Zone"}}, fp, history.New(5))

	next, _ := m.Update(streamResolvedMsg{
		channelTitle: "Drone Zone",
		streamURL:    "https://ice5.somafm.com/dronezone-128-mp3",
	})
	m = next.(Model)

	if m.nowPlaying.channel != "Drone Zone" {
		t.Fatalf("nowPlaying.channel = %q, want Drone Zone", m.nowPlaying.channel)
	}
	if m.nowPlaying.bitrate != 128 || m.nowPlaying.codec != "MP3" {
		t.Fatalf("nowPlaying bitrate/codec = %d/%s, want 128/MP3", m.nowPlaying.bitrate, m.nowPlaying.codec)
	}
	if urls := fp.PlayedURLs(); len(urls) != 1 || urls[0] != "https://ice5.somafm.com/dronezone-128-mp3" {
		t.Fatalf("fake player PlayedURLs() = %v, want the resolved stream URL", urls)
	}
}

func TestUpdate_TrackChangedMsgRecordsPreviousTrackToHistory(t *testing.T) {
	m := newTestModel()
	m.nowPlaying = nowPlayingState{title: "Old Track", artist: "Old Artist", channel: "Groove Salad"}

	next, _ := m.Update(player.TrackChangedMsg{Title: "New Track", Artist: "New Artist"})
	m = next.(Model)

	if m.nowPlaying.title != "New Track" || m.nowPlaying.artist != "New Artist" {
		t.Fatalf("nowPlaying = %+v, want New Track/New Artist", m.nowPlaying)
	}
	entries := m.hist.Entries()
	if len(entries) != 1 || entries[0].Title != "Old Track" {
		t.Fatalf("history entries = %+v, want the previous track recorded", entries)
	}
}

func TestUpdate_ConnectionLostAndReconnectedTogglesConnectedState(t *testing.T) {
	m := newTestModel()

	next, _ := m.Update(player.ConnectionLostMsg{})
	m = next.(Model)
	if m.nowPlaying.connected {
		t.Fatal("connected = true after ConnectionLostMsg, want false")
	}

	next, _ = m.Update(player.ReconnectedMsg{})
	m = next.(Model)
	if !m.nowPlaying.connected {
		t.Fatal("connected = false after ReconnectedMsg, want true")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/... -run "EnterOnChannel|StreamResolved|TrackChanged|ConnectionLost" -v`
Expected: FAIL (types/handlers undefined)

- [ ] **Step 3: Write playback.go**

`internal/ui/playback.go`:
```go
package ui

import (
	"context"
	"fmt"

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
		Title:   m.nowPlaying.title,
		Artist:  m.nowPlaying.artist,
		Channel: m.nowPlaying.channel,
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
			channel:   msg.channelTitle,
			bitrate:   bitrate,
			codec:     codec,
			connected: true,
		}
		m.errMsg = ""
		m.player.Play(msg.streamURL)
		return m, nil

	case player.TrackChangedMsg:
		m = m.recordCurrentTrackToHistory()
		m.nowPlaying.title = msg.Title
		m.nowPlaying.artist = msg.Artist
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
```

- [ ] **Step 4: Wire Init, Update, and the enter key into model.go**

Modify `internal/ui/model.go`: replace `Init` and extend `Update`.

```go
func (m Model) Init() tea.Cmd {
	return waitForPlayerMsg(m.player)
}
```

Add to the `case tea.KeyMsg` key switch in `Update`, alongside the existing cases:
```go
		case "enter":
			if ch, ok := m.selectedChannel(); ok {
				return m, resolveAndPlayCmd(ch)
			}
			return m, nil
```

Add before the final `return m, nil` at the end of `Update` (so non-key messages fall through to playback handling), and add the helper below `switchMode`:
```go
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
```

Replace the final line of `Update` (`return m, nil` at function scope, after the `switch msg := msg.(type)` block) with:
```go
	next, cmd := m.handlePlaybackMsg(msg)
	return next, tea.Batch(cmd, waitForPlayerMsg(m.player))
```

Note: `waitForPlayerMsg` should only be re-issued when the incoming message came from the player channel, not on every keystroke, otherwise duplicate listeners accumulate. Restrict the batch to player-originated messages:
```go
	switch msg.(type) {
	case player.TrackChangedMsg, player.ConnectionLostMsg, player.ReconnectedMsg:
		next, cmd := m.handlePlaybackMsg(msg)
		return next, tea.Batch(cmd, waitForPlayerMsg(m.player))
	}
	return m.handlePlaybackMsg(msg)
```
Place this as the final fallthrough in `Update`, after the `tea.KeyMsg` case.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/ui/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/ui/model.go internal/ui/playback.go internal/ui/playback_test.go
git commit -m "feat: wire channel playback, track-change history recording, and reconnect state"
```

---

### Task 11: Elapsed & session timers

**Files:**
- Create: `internal/ui/timers.go`
- Modify: `internal/ui/model.go`
- Test: `internal/ui/timers_test.go`

**Interfaces:**
- Produces: `formatDuration(d time.Duration) string`, `tickMsg time.Time`, `tickCmd() tea.Cmd`. `Model` gains `trackStarted`, `sessionStarted time.Time` and `elapsed`, `session string` fields on `nowPlayingState`/`Model` used by the view task.

- [ ] **Step 1: Write the failing test**

`internal/ui/timers_test.go`:
```go
package ui

import (
	"testing"
	"time"
)

func TestFormatDuration_FormatsMinutesAndSeconds(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0:00"},
		{92 * time.Second, "1:32"},
		{5 * time.Second, "0:05"},
		{125 * time.Minute, "125:00"},
	}
	for _, c := range cases {
		if got := formatDuration(c.d); got != c.want {
			t.Errorf("formatDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestUpdate_TickMsgUpdatesElapsedAndSession(t *testing.T) {
	m := newTestModel()
	now := time.Now()
	m.sessionStarted = now.Add(-90 * time.Second)
	m.nowPlaying.trackStarted = now.Add(-30 * time.Second)

	next, _ := m.Update(tickMsg(now))
	m = next.(Model)

	if m.nowPlaying.elapsed != "0:30" {
		t.Fatalf("elapsed = %q, want 0:30", m.nowPlaying.elapsed)
	}
	if m.session != "1:30" {
		t.Fatalf("session = %q, want 1:30", m.session)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/... -run "FormatDuration|TickMsg" -v`
Expected: FAIL (undefined symbols)

- [ ] **Step 3: Write timers.go**

`internal/ui/timers.go`:
```go
package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func (m Model) handleTick(now time.Time) Model {
	if !m.nowPlaying.trackStarted.IsZero() {
		m.nowPlaying.elapsed = formatDuration(now.Sub(m.nowPlaying.trackStarted))
	}
	if !m.sessionStarted.IsZero() {
		m.session = formatDuration(now.Sub(m.sessionStarted))
	}
	return m
}
```

- [ ] **Step 4: Wire timer fields and tick handling into model.go**

Modify `internal/ui/model.go`:
- Add `trackStarted time.Time` and `elapsed string` fields to `nowPlayingState`.
- Add `sessionStarted time.Time` and `session string` fields to `Model`.
- In `Init`, set the session start and start ticking:
```go
func (m Model) Init() tea.Cmd {
	return tea.Batch(waitForPlayerMsg(m.player), tickCmd())
}
```
  (`sessionStarted` must be set at construction time in `New`, not in `Init`, since `Init` runs on a copy: add `sessionStarted: time.Now()` to the struct literal returned by `New`.)
- In the fallthrough branch added in Task 10, handle `tickMsg` before the player-message batching:
```go
	if t, ok := msg.(tickMsg); ok {
		return m.handleTick(time.Time(t)), tickCmd()
	}
```
- In `handlePlaybackMsg`'s `streamResolvedMsg` and `player.TrackChangedMsg` cases, set `m.nowPlaying.trackStarted = time.Now()` whenever the title changes (channel switch or new track).

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/ui/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/ui/timers.go internal/ui/timers_test.go internal/ui/model.go
git commit -m "feat: add elapsed track time and session time ticking"
```

---

### Task 12: Volume and mute keys

**Files:**
- Create: `internal/ui/volume.go`
- Modify: `internal/ui/model.go`
- Test: `internal/ui/volume_test.go`

**Interfaces:**
- Consumes: `player.Player.SetVolume`, `player.Player.SetMuted` (Task 7); `config.Save` (Task 2).
- Produces: `(m Model) adjustVolume(delta int) Model`, `(m Model) toggleMute() Model`.

- [ ] **Step 1: Write the failing tests**

`internal/ui/volume_test.go`:
```go
package ui

import (
	"testing"

	"github.com/jonasbn/somafm-player/internal/player"
)

func TestUpdate_VolumeKeysAdjustInSteps(t *testing.T) {
	fp := player.NewFakePlayer()
	m := newTestModelWithPlayer(fp)
	m.cfg.Volume = 50

	next, _ := m.Update(key("+"))
	m = next.(Model)
	if m.cfg.Volume != 55 || fp.Volume() != 55 {
		t.Fatalf("volume after + = %d/%d, want 55/55", m.cfg.Volume, fp.Volume())
	}

	next, _ = m.Update(key("-"))
	m = next.(Model)
	if m.cfg.Volume != 50 || fp.Volume() != 50 {
		t.Fatalf("volume after - = %d/%d, want 50/50", m.cfg.Volume, fp.Volume())
	}
}

func TestUpdate_VolumeClampsBetweenZeroAndHundred(t *testing.T) {
	m := newTestModel()
	m.cfg.Volume = 98
	next, _ := m.Update(key("+"))
	next, _ = next.(Model).Update(key("+"))
	m = next.(Model)
	if m.cfg.Volume != 100 {
		t.Fatalf("volume = %d, want clamped at 100", m.cfg.Volume)
	}
}

func TestUpdate_MToggleMute(t *testing.T) {
	fp := player.NewFakePlayer()
	m := newTestModelWithPlayer(fp)

	next, _ := m.Update(key("m"))
	m = next.(Model)
	if !m.cfg.Muted || !fp.Muted() {
		t.Fatal("expected muted = true after m")
	}

	next, _ = m.Update(key("m"))
	m = next.(Model)
	if m.cfg.Muted || fp.Muted() {
		t.Fatal("expected muted = false after second m")
	}
}
```

Add the shared helper next to `newTestModel` in `internal/ui/model_test.go`:
```go
func newTestModelWithPlayer(p player.Player) Model {
	chs := []channels.Channel{{Title: "Groove Salad"}, {Title: "Drone Zone"}}
	return New(config.DefaultConfig(), chs, p, history.New(5))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/... -run Volume -v`
Expected: FAIL (`+`/`-`/`m` not handled)

- [ ] **Step 3: Write volume.go**

`internal/ui/volume.go`:
```go
package ui

func clampVolume(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func (m Model) adjustVolume(delta int) Model {
	m.cfg.Volume = clampVolume(m.cfg.Volume + delta)
	m.player.SetVolume(m.cfg.Volume)
	return m
}

func (m Model) toggleMute() Model {
	m.cfg.Muted = !m.cfg.Muted
	m.player.SetMuted(m.cfg.Muted)
	return m
}
```

- [ ] **Step 4: Wire the keys into model.go's key switch**

Add to the key switch in `Update`:
```go
		case "+", "right":
			return m.adjustVolume(5), nil
		case "-", "left":
			return m.adjustVolume(-5), nil
		case "m":
			return m.toggleMute(), nil
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/ui/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/ui/volume.go internal/ui/volume_test.go internal/ui/model.go internal/ui/model_test.go
git commit -m "feat: add volume adjustment and mute toggle keys"
```

---

### Task 13: Bookmark `b` context-sensitive wiring

**Files:**
- Create: `internal/ui/bookmark_actions.go`
- Modify: `internal/ui/model.go`
- Test: `internal/ui/bookmark_actions_test.go`

**Interfaces:**
- Consumes: `bookmarks.ToggleChannel`, `bookmarks.AddTune` (Task 6); `config.BookmarkedTune` (Task 2).
- Produces: `(m Model) handleBookmarkKey() Model`.

- [ ] **Step 1: Write the failing tests**

`internal/ui/bookmark_actions_test.go`:
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

func TestBookmarkKey_OnChannelsListTogglesSelectedChannel(t *testing.T) {
	m := newTestModel()
	m.focus = focusList
	m.mode = viewChannels
	m.selected = 0 // "Groove Salad"

	m = m.handleBookmarkKey()
	if len(m.cfg.BookmarkedChannels) != 1 || m.cfg.BookmarkedChannels[0] != "Groove Salad" {
		t.Fatalf("BookmarkedChannels = %v, want [Groove Salad]", m.cfg.BookmarkedChannels)
	}

	m = m.handleBookmarkKey()
	if len(m.cfg.BookmarkedChannels) != 0 {
		t.Fatalf("BookmarkedChannels = %v, want empty after second toggle", m.cfg.BookmarkedChannels)
	}
}

func TestBookmarkKey_OnHistoryBookmarksSelectedEntryAsTune(t *testing.T) {
	m := newTestModel()
	m.focus = focusList
	m.mode = viewHistory
	m.hist.Add(historyEntry("Old Song", "Old Artist", "Drone Zone"))
	m.selected = 0

	m = m.handleBookmarkKey()

	if len(m.cfg.BookmarkedTunes) != 1 || m.cfg.BookmarkedTunes[0].Title != "Old Song" {
		t.Fatalf("BookmarkedTunes = %+v, want the selected history entry bookmarked", m.cfg.BookmarkedTunes)
	}
}
```

Add this small test helper to `internal/ui/model_test.go` (used only by the history-focused test above):
```go
func historyEntry(title, artist, channel string) history.Entry {
	return history.Entry{Title: title, Artist: artist, Channel: channel}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/... -run BookmarkKey -v`
Expected: FAIL (`handleBookmarkKey` undefined)

- [ ] **Step 3: Write bookmark_actions.go**

`internal/ui/bookmark_actions.go`:
```go
package ui

import (
	"github.com/jonasbn/somafm-player/internal/bookmarks"
	"github.com/jonasbn/somafm-player/internal/config"
)

func (m Model) handleBookmarkKey() Model {
	if m.focus == focusNowPlaying {
		if m.nowPlaying.title == "" {
			return m
		}
		bookmarks.AddTune(&m.cfg, config.BookmarkedTune{
			Title:   m.nowPlaying.title,
			Artist:  m.nowPlaying.artist,
			Channel: m.nowPlaying.channel,
		})
		return m
	}

	switch m.mode {
	case viewChannels:
		if ch, ok := m.selectedChannel(); ok {
			bookmarks.ToggleChannel(&m.cfg, ch.Title)
		}
	case viewBookmarkedChannels:
		if m.selected < len(m.cfg.BookmarkedChannels) {
			bookmarks.ToggleChannel(&m.cfg, m.cfg.BookmarkedChannels[m.selected])
		}
	case viewHistory:
		entries := m.hist.Entries()
		if m.selected < len(entries) {
			e := entries[m.selected]
			bookmarks.AddTune(&m.cfg, config.BookmarkedTune{Title: e.Title, Artist: e.Artist, Channel: e.Channel})
		}
	case viewBookmarkedTunes:
		// already bookmarked; no-op
	}
	return m
}
```

- [ ] **Step 4: Wire the `b` key into model.go**

Add to the key switch in `Update`:
```go
		case "b":
			return m.handleBookmarkKey(), nil
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/ui/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/ui/bookmark_actions.go internal/ui/bookmark_actions_test.go internal/ui/model.go internal/ui/model_test.go
git commit -m "feat: add context-sensitive bookmark key for tunes and channels"
```

---

### Task 14: Theme cycling

**Files:**
- Create: `internal/ui/theme_actions.go`
- Modify: `internal/ui/model.go`
- Test: `internal/ui/theme_actions_test.go`

**Interfaces:**
- Consumes: `theme.Next`, `theme.Get` (Task 3).
- Produces: `(m Model) cycleTheme() Model`.

- [ ] **Step 1: Write the failing test**

`internal/ui/theme_actions_test.go`:
```go
package ui

import "testing"

func TestUpdate_TKeyCyclesTheme(t *testing.T) {
	m := newTestModel()
	m.cfg.Theme = "Nord"

	next, _ := m.Update(key("t"))
	m = next.(Model)
	if m.cfg.Theme != "Dracula" {
		t.Fatalf("theme after t = %q, want Dracula", m.cfg.Theme)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/... -run TKeyCyclesTheme -v`
Expected: FAIL (`t` not handled)

- [ ] **Step 3: Write theme_actions.go**

`internal/ui/theme_actions.go`:
```go
package ui

import "github.com/jonasbn/somafm-player/internal/theme"

func (m Model) cycleTheme() Model {
	m.cfg.Theme = theme.Next(m.cfg.Theme)
	return m
}
```

- [ ] **Step 4: Wire the `t` key into model.go**

Add to the key switch in `Update`:
```go
		case "t":
			return m.cycleTheme(), nil
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/ui/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/ui/theme_actions.go internal/ui/theme_actions_test.go internal/ui/model.go
git commit -m "feat: add theme cycling key"
```

---

### Task 15: View rendering — Now Playing and list panels

**Files:**
- Create: `internal/ui/view.go`
- Modify: `internal/ui/model.go`

**Interfaces:**
- Consumes: `theme.Get`, `theme.Theme` (Task 3); all `Model` state from prior tasks.
- Produces: `Model.View() string` fully implemented (no automated test — rendering is verified visually per Step 3).

- [ ] **Step 1: Write view.go**

`internal/ui/view.go`:
```go
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jonasbn/somafm-player/internal/bookmarks"
	"github.com/jonasbn/somafm-player/internal/theme"
)

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
	filled := m.cfg.Volume / 10
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)
	label := fmt.Sprintf("Vol: %s %d%%", bar, m.cfg.Volume)
	if m.cfg.Muted {
		label += " (muted)"
	}
	return label
}

func (m Model) listHeader() string {
	labels := []string{"Channels", "Bookmarked Channels", "Bookmarked Tunes", "History"}
	for i, l := range labels {
		if viewMode(i) == m.mode {
			labels[i] = "[" + l + "]"
		}
	}
	return strings.Join(labels, " ▸ ")
}

func (m Model) listLines() []string {
	switch m.mode {
	case viewChannels:
		lines := make([]string, len(m.channels))
		for i, ch := range m.channels {
			mark := "  "
			if bookmarks.IsChannelBookmarked(m.cfg, ch.Title) {
				mark = "★ "
			}
			lines[i] = fmt.Sprintf("%s%-24s %s", mark, ch.Title, ch.Genre)
		}
		return lines
	case viewBookmarkedChannels:
		lines := make([]string, len(m.cfg.BookmarkedChannels))
		copy(lines, m.cfg.BookmarkedChannels)
		return lines
	case viewBookmarkedTunes:
		lines := make([]string, len(m.cfg.BookmarkedTunes))
		for i, t := range m.cfg.BookmarkedTunes {
			lines[i] = fmt.Sprintf("%s — %s (%s)", t.Title, t.Artist, t.Channel)
		}
		return lines
	case viewHistory:
		entries := m.hist.Entries()
		lines := make([]string, len(entries))
		for i, e := range entries {
			lines[i] = fmt.Sprintf("%s — %s (%s) @ %s", e.Title, e.Artist, e.Channel, e.PlayedAt.Format("15:04:05"))
		}
		return lines
	}
	return nil
}

func (m Model) renderList(t theme.Theme) string {
	lines := m.listLines()
	rendered := make([]string, 0, len(lines)+1)
	rendered = append(rendered, m.listHeader())
	if len(lines) == 0 {
		rendered = append(rendered, "(empty)")
	}
	for i, line := range lines {
		prefix := "  "
		if i == m.selected {
			prefix = "> "
		}
		rendered = append(rendered, prefix+line)
	}
	return borderStyle(t, m.focus == focusList).Render(strings.Join(rendered, "\n"))
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	t := theme.Get(m.cfg.Theme)

	footer := fmt.Sprintf("[Theme: %s]  tab focus · j/k move · enter play · b bookmark · c/f/s/H panels · +/- vol · m mute · t theme · q quit", t.Name)
	if m.errMsg != "" {
		footer = "Error: " + m.errMsg + "\n" + footer
	}

	return strings.Join([]string{
		m.renderNowPlaying(t),
		m.renderList(t),
		footer,
	}, "\n")
}
```

- [ ] **Step 2: Remove the placeholder `View` left in model.go**

Modify `internal/ui/model.go`: delete the existing `func (m Model) View() string { ... }` body (now defined in `view.go`).

- [ ] **Step 3: Build and manually verify rendering**

Run: `go build ./... && go run .`
Expected: the Now Playing panel, list panel with header/selection marker, and footer render without panics; `tab`, `j/k`, `c/f/s/H`, `t`, `+/-`, `m`, `b`, `q` all behave as designed when tried by hand against a live channel list (network required for the channel fetch in `main.go`).

- [ ] **Step 4: Commit**

```bash
git add internal/ui/view.go internal/ui/model.go
git commit -m "feat: render Now Playing and list panels with theme styling"
```

---

### Task 16: Startup wiring — auto-resume, error state, save on exit

**Files:**
- Modify: `main.go`
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/playback.go`

**Interfaces:**
- Consumes: `config.Load`, `config.Save` (Task 2); `channels.Fetch` (Task 4).
- Produces: `Model.Init` auto-plays `cfg.LastChannel` when present; config is saved on quit.

- [ ] **Step 1: Add auto-resume to Init**

Modify `internal/ui/model.go`'s `Init`:
```go
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
```

- [ ] **Step 2: Persist `lastChannel` whenever a channel starts playing**

Modify `internal/ui/playback.go`'s `streamResolvedMsg` case in `handlePlaybackMsg`, immediately after setting `m.nowPlaying`:
```go
		m.cfg.LastChannel = msg.channelTitle
```

- [ ] **Step 3: Save config on quit**

Modify `internal/ui/model.go`'s quit handling to save synchronously before quitting:
```go
		case "q", "ctrl+c":
			m.quitting = true
			_ = config.Save(m.cfg)
			return m, tea.Quit
```
(add `"github.com/jonasbn/somafm-player/internal/config"` to the import block if not already present from other tasks)

- [ ] **Step 4: Handle channels.json fetch failure without crashing, in main.go**

Modify `main.go`: on `channels.Fetch` error, still start the UI with an empty channel list and an inline error, rather than exiting the process.
```go
	chs, fetchErr := channels.Fetch(context.Background(), channelsURL)

	m := ui.New(cfg, chs, player.NewRealPlayer(), history.New(5))
	if fetchErr != nil {
		m = m.WithStartupError(fmt.Sprintf("Couldn't load channel list — check your connection, press r to retry (%v)", fetchErr))
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
```

Add the small exported setter to `internal/ui/model.go` (used only from `main.go`):
```go
func (m Model) WithStartupError(msg string) Model {
	m.errMsg = msg
	return m
}
```

- [ ] **Step 5: Build and verify**

Run: `go build ./... && go test ./... -v`
Expected: clean build, all tests pass

- [ ] **Step 6: Commit**

```bash
git add main.go internal/ui/model.go internal/ui/playback.go
git commit -m "feat: auto-resume last channel, persist config, handle channel-list fetch failure"
```

---

### Task 17: Manual end-to-end smoke test and README usage notes

**Files:**
- Modify: `README.md`

**Interfaces:** None (documentation + manual verification only).

- [ ] **Step 1: Run the full test suite**

Run: `go test ./... -v`
Expected: PASS across all packages

- [ ] **Step 2: Manual smoke test against the real SomaFM service**

Run: `go run .`
Walk through, confirming each behaves as designed:
- App launches, fetches the live channel list, lands on the Channels panel (or auto-resumes if `~/.config/somafm-player/config.json` already has a `lastChannel` from a previous run).
- `j`/`k` move selection; `enter` plays a channel; audio is audible within a few seconds.
- Now Playing panel shows title/artist (once ICY metadata arrives), channel, bitrate/codec, elapsed/session time, and the volume bar.
- `+`/`-` change volume audibly; `m` mutes/unmutes.
- `b` on the channel list bookmarks it (★ appears); `f` shows it in Bookmarked Channels; `b` again on Now Playing bookmarks the current tune, visible via `s`.
- `H` shows history populating as tracks change.
- `t` cycles all six themes visibly.
- Disconnect network briefly (e.g. toggle Wi-Fi off/on) and confirm "Reconnecting…" appears and playback resumes automatically without crashing.
- `q` exits cleanly; relaunching `go run .` auto-resumes the last channel and restores the saved volume/theme/bookmarks (history is expected to be empty again, per spec).

- [ ] **Step 3: Add a Usage section to README.md**

Add after the existing "## Stack" section in `README.md`:
```markdown
## Usage

```
go run .
```

| Key | Action |
|---|---|
| `tab` | toggle focus between Now Playing and the list panel |
| `j`/`k` or arrows | move selection |
| `enter` | play selected channel |
| `c` / `f` / `s` / `H` | switch list panel: Channels / Bookmarked Channels / Bookmarked Tunes / History |
| `b` | bookmark (context-sensitive: tune on Now Playing, channel on Channels/Bookmarked Channels, tune on History) |
| `+`/`-` or arrows | volume up/down |
| `m` | mute/unmute |
| `t` | cycle theme |
| `q` | quit |

Config, including bookmarks, volume, theme, and last-played channel, is stored at
`~/.config/somafm-player/config.json` (or under `$XDG_CONFIG_HOME` if set).
```

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: add usage and keybindings section"
```
