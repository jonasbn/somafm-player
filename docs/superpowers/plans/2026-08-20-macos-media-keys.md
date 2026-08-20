# macOS Media Key (Play/Pause) Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the hardware Play/Pause key on macOS toggle mute in somafm-player, and make Control Center / the lock screen show the current channel and track.

**Architecture:** A new `internal/mediakeys` package exposes a small `Controller` interface (`Events()`, `SetNowPlaying()`, `SetPlaying()`, `Close()`). The darwin build (`//go:build darwin`) implements it with a cgo/Objective-C bridge to `MediaPlayer.framework` (`MPRemoteCommandCenter` for receiving key events, `MPNowPlayingInfoCenter` for publishing state), running a dedicated goroutine that pumps a `CFRunLoop`/`NSRunLoop` on a locked OS thread. A `!darwin` build provides a no-op stub. `internal/ui` wires the controller in through `Model` exactly the way `player.Player` is already wired in: a `waitForMediaKeyCmd` matching `waitForPlayerMsg`, and a `FakeController` matching `FakePlayer` for tests.

**Tech Stack:** Go 1.25, Bubble Tea, cgo, Objective-C, `MediaPlayer.framework`/`Foundation.framework` (macOS system frameworks, no new Go dependencies).

**Spec:** `docs/superpowers/specs/2026-08-20-macos-media-keys-design.md`

## Global Constraints

- Darwin-only feature; `!darwin` builds must still compile via a no-op stub (project already builds darwin-only in `.goreleaser.yaml` with `CGO_ENABLED=1`, so no new cross-compilation burden is introduced).
- "Pause" means mute, not stream disconnect — `Play`/`Pause`/toggle all resolve to the existing `m.toggleMute()` behavior. No new playback semantics.
- No Next/Previous-track handling — only `playCommand`/`pauseCommand`/`togglePlayPauseCommand` are registered.
- No "resume last channel" behavior — if nothing is playing, there's nothing for the key to do.
- `FakeController` (in `internal/mediakeys/fake.go`, not a `_test.go` file) must mirror the existing `player.FakePlayer` pattern exactly, since `internal/ui` tests need to import it across package boundaries.
- The cgo/Objective-C bridge itself (Task 3) cannot be exercised by an automated test in this environment — per `CLAUDE.md`, there is no TTY/audio/media-key hardware available to agents. It is verified by compiling only; a human must confirm the actual key-press behavior on real hardware.
- CI runs on `macos-latest` (`.github/workflows/pr-main-build-test-lint.yml`), so `go build`/`go vet`/lint will catch real cgo/ObjC compile errors even though interactive behavior can't be asserted there.

---

## Task 1: `internal/mediakeys` core types and `FakeController`

**Files:**
- Create: `internal/mediakeys/mediakeys.go`
- Create: `internal/mediakeys/fake.go`
- Create: `internal/mediakeys/fake_test.go`

**Interfaces:**
- Produces: `type Event int` with const `PlayPauseEvent`; `type NowPlayingInfo struct { Channel, Title, Artist string }`; `type Controller interface { Events() <-chan Event; SetNowPlaying(NowPlayingInfo); SetPlaying(bool); Close() }`; `type FakeController struct{...}` with `NewFakeController() *FakeController`, `(*FakeController) Emit(Event)`, `(*FakeController) NowPlaying() NowPlayingInfo`, `(*FakeController) Playing() bool`, `(*FakeController) Closed() bool` (all satisfying `Controller`).

- [ ] **Step 1: Write the failing test for `FakeController`**

Create `internal/mediakeys/fake_test.go`:

```go
package mediakeys

import "testing"

func TestFakeController_EmitDeliversOnEventsChannel(t *testing.T) {
	fc := NewFakeController()

	fc.Emit(PlayPauseEvent)

	got := <-fc.Events()
	if got != PlayPauseEvent {
		t.Fatalf("Events() delivered %v, want PlayPauseEvent", got)
	}
}

func TestFakeController_TracksNowPlayingAndPlayingState(t *testing.T) {
	fc := NewFakeController()

	fc.SetNowPlaying(NowPlayingInfo{Channel: "Groove Salad", Title: "Song", Artist: "Band"})
	fc.SetPlaying(true)

	want := NowPlayingInfo{Channel: "Groove Salad", Title: "Song", Artist: "Band"}
	if got := fc.NowPlaying(); got != want {
		t.Fatalf("NowPlaying() = %+v, want %+v", got, want)
	}
	if !fc.Playing() {
		t.Fatal("Playing() = false, want true")
	}
}

func TestFakeController_CloseMarksClosed(t *testing.T) {
	fc := NewFakeController()
	fc.Close()
	if !fc.Closed() {
		t.Fatal("Closed() = false after Close(), want true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mediakeys/... -run TestFakeController -v`
Expected: FAIL — `mediakeys.NewFakeController`, `mediakeys.PlayPauseEvent`, `mediakeys.NowPlayingInfo` undefined (package doesn't exist yet).

- [ ] **Step 3: Write `internal/mediakeys/mediakeys.go`**

```go
// Package mediakeys integrates somafm-player with macOS hardware media
// keys (play/pause) via MPRemoteCommandCenter/MPNowPlayingInfoCenter. The
// darwin implementation lives in mediakeys_darwin.go; mediakeys_other.go
// provides a no-op stub for every other OS.
package mediakeys

// Event identifies a media-key action delivered by a Controller.
type Event int

// PlayPauseEvent is sent when the hardware play, pause, or toggle
// play/pause key is pressed. somafm-player has no true "pause" for a
// live stream, so all three map to the same event and the UI layer
// treats it as a mute toggle.
const PlayPauseEvent Event = iota

// NowPlayingInfo is the metadata published to macOS so Control Center,
// the menu-bar Now Playing widget, and the lock screen can display it.
type NowPlayingInfo struct {
	Channel string
	Title   string
	Artist  string
}

// Controller receives hardware media-key events and publishes playback
// state to macOS. Callers must call Close when done.
type Controller interface {
	// Events delivers a PlayPauseEvent each time the hardware key is
	// pressed. The channel is never closed by the Controller.
	Events() <-chan Event
	// SetNowPlaying updates the metadata shown by macOS. Empty fields
	// are treated as "unknown" and omitted from the display.
	SetNowPlaying(info NowPlayingInfo)
	// SetPlaying updates the playback state shown by macOS.
	SetPlaying(playing bool)
	// Close unregisters the controller and releases its resources.
	Close()
}
```

- [ ] **Step 4: Write `internal/mediakeys/fake.go`**

```go
package mediakeys

import "sync"

// FakeController is a Controller test double, mirroring
// player.FakePlayer: tests use Emit to simulate a hardware key press,
// and NowPlaying/Playing/Closed to assert what the code under test
// published.
type FakeController struct {
	mu         sync.Mutex
	events     chan Event
	nowPlaying NowPlayingInfo
	playing    bool
	closed     bool
}

func NewFakeController() *FakeController {
	return &FakeController{events: make(chan Event, 4)}
}

func (f *FakeController) Events() <-chan Event {
	return f.events
}

func (f *FakeController) SetNowPlaying(info NowPlayingInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nowPlaying = info
}

func (f *FakeController) SetPlaying(playing bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.playing = playing
}

func (f *FakeController) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
}

// Emit simulates a hardware media-key press, delivering e on Events().
func (f *FakeController) Emit(e Event) {
	f.events <- e
}

func (f *FakeController) NowPlaying() NowPlayingInfo {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.nowPlaying
}

func (f *FakeController) Playing() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.playing
}

func (f *FakeController) Closed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/mediakeys/... -v`
Expected: PASS (all three `TestFakeController_*` tests).

- [ ] **Step 6: Commit**

```bash
git add internal/mediakeys/mediakeys.go internal/mediakeys/fake.go internal/mediakeys/fake_test.go
git commit -m "feat(mediakeys): add Controller interface and FakeController test double"
```

---

## Task 2: `!darwin` no-op stub

**Files:**
- Create: `internal/mediakeys/mediakeys_other.go`

**Interfaces:**
- Consumes: `Controller`, `Event`, `NowPlayingInfo` from Task 1.
- Produces: `func New() (Controller, error)` (the `!darwin` variant — Task 3 provides the darwin variant with the identical signature).

This package has no automated test on this repo's CI (which runs `macos-latest` exclusively), so there is no failing-test step here — the implementation is a handful of no-ops, and correctness is checked by cross-compiling it in isolation.

- [ ] **Step 1: Write `internal/mediakeys/mediakeys_other.go`**

```go
//go:build !darwin

package mediakeys

// New returns a Controller that does nothing: no hardware key events are
// ever delivered, and SetNowPlaying/SetPlaying/Close are no-ops. macOS
// media-key integration only exists on darwin; every other OS gets this
// stub so the rest of the module stays portable.
func New() (Controller, error) {
	return &noopController{events: make(chan Event)}, nil
}

type noopController struct {
	events chan Event
}

func (c *noopController) Events() <-chan Event           { return c.events }
func (c *noopController) SetNowPlaying(NowPlayingInfo)    {}
func (c *noopController) SetPlaying(bool)                 {}
func (c *noopController) Close()                          {}
```

- [ ] **Step 2: Verify it compiles on a non-darwin target**

Run: `GOOS=linux GOARCH=amd64 go vet ./internal/mediakeys/`
Expected: no output (success) — this checks only the `mediakeys` package in isolation, since the rest of the module (`internal/player/real.go`'s audio backend) isn't guaranteed to cross-compile and isn't this task's concern.

- [ ] **Step 3: Verify the darwin build of the package still compiles (this file must NOT be picked up on darwin)**

Run: `go build ./internal/mediakeys/...`
Expected: succeeds. (At this point in the plan `New` is only defined for `!darwin`; since we're building on darwin — this repo's actual dev/CI OS — Go's build constraints exclude `mediakeys_other.go` automatically by the `!darwin` tag, so this command currently fails with "New redeclared" only if both files define it — it won't, since Task 3 hasn't been written yet, so on darwin right now there is no `New` at all and nothing references the package yet. This step just confirms `mediakeys.go` and `mediakeys_other.go` together don't have any syntax errors reachable from a darwin build.)

- [ ] **Step 4: Commit**

```bash
git add internal/mediakeys/mediakeys_other.go
git commit -m "feat(mediakeys): add no-op Controller for non-darwin builds"
```

---

## Task 3: darwin cgo/Objective-C bridge

**Files:**
- Create: `internal/mediakeys/bridge_darwin.h`
- Create: `internal/mediakeys/bridge_darwin.m`
- Create: `internal/mediakeys/mediakeys_darwin.go`

**Interfaces:**
- Consumes: `Controller`, `Event`, `PlayPauseEvent`, `NowPlayingInfo` from Task 1.
- Produces: `func New() (Controller, error)` (the darwin variant, same signature as Task 2's stub — only one of the two is ever compiled into a given binary).

This is the one component the spec designates as **not unit-testable**: it talks to real macOS system services (`MPRemoteCommandCenter`, `MPNowPlayingInfoCenter`) that have no headless/mock equivalent, and this environment has no way to simulate a hardware key press. There is no failing-test step — verification is (a) it compiles, and (b) a manual check on real hardware, spelled out at the end.

- [ ] **Step 1: Write `internal/mediakeys/bridge_darwin.h`**

```c
#ifndef SOMAFM_MEDIAKEYS_BRIDGE_DARWIN_H
#define SOMAFM_MEDIAKEYS_BRIDGE_DARWIN_H

// mediakeys_start registers this process as a Now Playing target: it
// installs handlers on MPRemoteCommandCenter's play/pause/toggle
// commands that call back into Go via goMediaKeyPlayPause (see
// mediakeys_darwin.go's //export directive and the generated
// _cgo_export.h).
void mediakeys_start(void);

// mediakeys_run_loop pumps the current thread's run loop forever. The
// command handlers registered by mediakeys_start only fire while a run
// loop is being pumped, so this must run on its own goroutine for the
// lifetime of the process (see mediakeys_darwin.go's New).
void mediakeys_run_loop(void);

// mediakeys_stop unregisters the command handlers installed by
// mediakeys_start.
void mediakeys_stop(void);

// mediakeys_set_now_playing publishes channel/title/artist to
// MPNowPlayingInfoCenter. title and artist may be empty strings, in
// which case they're omitted (channel is used as the displayed title).
void mediakeys_set_now_playing(const char *channel, const char *title, const char *artist);

// mediakeys_set_playing updates the playback state shown by macOS.
// playing is a C bool encoded as 0/1.
void mediakeys_set_playing(int playing);

#endif
```

- [ ] **Step 2: Write `internal/mediakeys/bridge_darwin.m`**

```objective-c
#import "bridge_darwin.h"
#import <Foundation/Foundation.h>
#import <MediaPlayer/MediaPlayer.h>
#import "_cgo_export.h"

void mediakeys_start(void) {
    MPRemoteCommandCenter *center = [MPRemoteCommandCenter sharedCommandCenter];

    MPRemoteCommandHandler handler = ^MPRemoteCommandHandlerStatus(MPRemoteCommandEvent *event) {
        goMediaKeyPlayPause();
        return MPRemoteCommandHandlerStatusSuccess;
    };

    [center.playCommand addTargetWithHandler:handler];
    [center.pauseCommand addTargetWithHandler:handler];
    [center.togglePlayPauseCommand addTargetWithHandler:handler];

    center.playCommand.enabled = YES;
    center.pauseCommand.enabled = YES;
    center.togglePlayPauseCommand.enabled = YES;
}

void mediakeys_run_loop(void) {
    // [[NSRunLoop currentRunLoop] run] returns as soon as the loop has no
    // scheduled input sources, which can happen depending on how the
    // command center wires up its internal XPC connection. Looping
    // runMode:beforeDate: with a distant-future date is the standard
    // idiom for pumping a background thread's run loop forever.
    while (1) {
        [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode
                                  beforeDate:[NSDate distantFuture]];
    }
}

void mediakeys_stop(void) {
    MPRemoteCommandCenter *center = [MPRemoteCommandCenter sharedCommandCenter];
    [center.playCommand removeTarget:nil];
    [center.pauseCommand removeTarget:nil];
    [center.togglePlayPauseCommand removeTarget:nil];
}

void mediakeys_set_now_playing(const char *channel, const char *title, const char *artist) {
    NSMutableDictionary *info = [NSMutableDictionary dictionary];

    NSString *channelStr = [NSString stringWithUTF8String:channel];
    NSString *titleStr = (title != NULL && title[0] != '\0') ? [NSString stringWithUTF8String:title] : nil;
    NSString *artistStr = (artist != NULL && artist[0] != '\0') ? [NSString stringWithUTF8String:artist] : nil;

    info[MPMediaItemPropertyTitle] = titleStr ?: channelStr;
    if (artistStr != nil) {
        info[MPMediaItemPropertyArtist] = artistStr;
    }

    [MPNowPlayingInfoCenter defaultCenter].nowPlayingInfo = info;
}

void mediakeys_set_playing(int playing) {
    [MPNowPlayingInfoCenter defaultCenter].playbackState =
        playing ? MPNowPlayingPlaybackStatePlaying : MPNowPlayingPlaybackStatePaused;
}
```

- [ ] **Step 3: Write `internal/mediakeys/mediakeys_darwin.go`**

```go
//go:build darwin

package mediakeys

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework MediaPlayer -framework Foundation
#include "bridge_darwin.h"
#include <stdlib.h>
*/
import "C"

import (
	"runtime"
	"unsafe"
)

// activeEvents is the single darwin Controller's event channel.
// mediakeys.New is only ever called once per process (from main.go), so
// a package-level channel is simpler than threading a context pointer
// through the cgo callback boundary.
var activeEvents = make(chan Event, 1)

//export goMediaKeyPlayPause
func goMediaKeyPlayPause() {
	// Non-blocking send: this runs on the ObjC run-loop thread inside a
	// command handler callback and must never block waiting for Go to
	// catch up, mirroring internal/spectrum.Analyzer.Feed's send.
	select {
	case activeEvents <- PlayPauseEvent:
	default:
	}
}

type darwinController struct {
	events chan Event
}

// New registers this process as a Now Playing target and starts the
// background run-loop goroutine that keeps the registration alive.
func New() (Controller, error) {
	go func() {
		runtime.LockOSThread()
		C.mediakeys_start()
		C.mediakeys_run_loop() // never returns
	}()

	return &darwinController{events: activeEvents}, nil
}

func (c *darwinController) Events() <-chan Event {
	return c.events
}

func (c *darwinController) SetNowPlaying(info NowPlayingInfo) {
	cChannel := C.CString(info.Channel)
	cTitle := C.CString(info.Title)
	cArtist := C.CString(info.Artist)
	defer C.free(unsafe.Pointer(cChannel))
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cArtist))

	C.mediakeys_set_now_playing(cChannel, cTitle, cArtist)
}

func (c *darwinController) SetPlaying(playing bool) {
	v := C.int(0)
	if playing {
		v = C.int(1)
	}
	C.mediakeys_set_playing(v)
}

func (c *darwinController) Close() {
	C.mediakeys_stop()
}
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./internal/mediakeys/...`
Expected: succeeds with no output. This is a real compile of Objective-C against `MediaPlayer.framework`/`Foundation.framework` via cgo — if the framework headers, the `_cgo_export.h` symbol name, or any Objective-C syntax is wrong, this fails here.

- [ ] **Step 5: Verify the whole module still builds and existing tests still pass**

Run: `go build ./... && go test ./...`
Expected: builds clean; all existing tests pass (nothing in this task touches `internal/ui` or `main.go` yet, so no behavior changes anywhere else).

- [ ] **Step 6: Commit**

```bash
git add internal/mediakeys/bridge_darwin.h internal/mediakeys/bridge_darwin.m internal/mediakeys/mediakeys_darwin.go
git commit -m "feat(mediakeys): add darwin MPRemoteCommandCenter/MPNowPlayingInfoCenter bridge"
```

- [ ] **Step 7 (manual verification — cannot be automated, do this yourself on a real Mac after the full feature is wired up through Task 4):**

1. `go run .` in a real terminal (not this sandbox).
2. Press `enter` on a channel to start playback.
3. Press the hardware Play/Pause key (or the Touch Bar / menu-bar media control). Confirm the app mutes (same visible effect as pressing `m`).
4. Press it again. Confirm it unmutes.
5. Open Control Center (or the menu-bar Now Playing widget). Confirm the channel/track title is shown.
6. This step can't run until Task 4 wires the controller into the UI — note it here now, re-run it after Task 4 and again after Task 5 (metadata should appear starting with Task 5).

---

## Task 4: Wire `Controller` into `ui.Model` — key press toggles mute

**Files:**
- Create: `internal/ui/media_keys.go`
- Create: `internal/ui/media_keys_test.go`
- Modify: `internal/ui/model.go:51-95` (struct, `New`), `internal/ui/model.go:97-111` (`Init`), `internal/ui/model.go:255-262` (`Update` dispatch)
- Modify: `internal/ui/model_test.go:14-25,56,220,249` (all `New(...)` call sites)
- Modify: `internal/ui/playback_test.go:53` (`New(...)` call site)
- Modify: `main.go:19-45`

**Interfaces:**
- Consumes: `mediakeys.Controller`, `mediakeys.New`, `mediakeys.NewFakeController` (Tasks 1-3).
- Produces: `Model.mediaKeys mediakeys.Controller` field; `New(cfg config.Config, chs []channels.Channel, p player.Player, hist *history.History, mk mediakeys.Controller) Model` (signature change — 5th parameter added); `waitForMediaKeyCmd(c mediakeys.Controller) tea.Cmd`; `(m Model) handleMediaKeyMsg() Model` (later tasks/other files can call this).

- [ ] **Step 1: Write the failing test**

Create `internal/ui/media_keys_test.go`:

```go
package ui

import (
	"testing"

	"github.com/jonasbn/somafm-player/internal/mediakeys"
	"github.com/jonasbn/somafm-player/internal/player"
)

func TestUpdate_MediaKeyEventTogglesMute(t *testing.T) {
	fp := player.NewFakePlayer()
	mk := mediakeys.NewFakeController()
	m := newTestModelWithPlayerAndMediaKeys(fp, mk)

	mk.Emit(mediakeys.PlayPauseEvent)
	cmd := waitForMediaKeyCmd(mk)
	next, _ := m.Update(cmd())
	m = next.(Model)

	if !m.cfg.Muted || !fp.Muted() {
		t.Fatal("expected muted = true after a media-key event")
	}

	mk.Emit(mediakeys.PlayPauseEvent)
	next, _ = m.Update(cmd())
	m = next.(Model)

	if m.cfg.Muted || fp.Muted() {
		t.Fatal("expected muted = false after a second media-key event")
	}
}

func TestWaitForMediaKeyCmd_ReturnsMediaKeyMsgWhenEventArrives(t *testing.T) {
	mk := mediakeys.NewFakeController()
	mk.Emit(mediakeys.PlayPauseEvent)

	msg := waitForMediaKeyCmd(mk)()

	if _, ok := msg.(mediaKeyMsg); !ok {
		t.Fatalf("waitForMediaKeyCmd() = %T, want mediaKeyMsg", msg)
	}
}
```

This references a `newTestModelWithPlayerAndMediaKeys` helper that doesn't exist yet — add it to `model_test.go` in the same step (test helpers are infrastructure for the test, not production code, so it's written alongside rather than in a separate step):

In `internal/ui/model_test.go`, add next to the existing `newTestModelWithPlayer` (after line 25):

```go
func newTestModelWithPlayerAndMediaKeys(p player.Player, mk mediakeys.Controller) Model {
	chs := []channels.Channel{{Title: "Groove Salad"}, {Title: "Drone Zone"}}
	return New(config.DefaultConfig(), chs, p, history.New(5), mk)
}
```

And add the import to `model_test.go`'s import block (it currently imports `channels`, `config`, `history`, `player`, plus `tea` and `testing`/`time` — add `"github.com/jonasbn/somafm-player/internal/mediakeys"` alongside them).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/... -run 'TestUpdate_MediaKeyEventTogglesMute|TestWaitForMediaKeyCmd' -v`
Expected: FAIL to compile — `waitForMediaKeyCmd`, `mediaKeyMsg` undefined, and `New` called with 5 args when it only takes 4.

- [ ] **Step 3: Write `internal/ui/media_keys.go`**

```go
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
```

- [ ] **Step 4: Modify `internal/ui/model.go` — struct field and `New`**

In the `Model` struct (`model.go:51-72`), add the field next to `player`/`hist`:

```go
	player    player.Player
	hist      *history.History
	mediaKeys mediakeys.Controller
```

Change `New` (`model.go:74-95`) to accept and store it:

```go
func New(cfg config.Config, chs []channels.Channel, p player.Player, hist *history.History, mk mediakeys.Controller) Model {
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
		mediaKeys:      mk,
		width:          defaultWidth,
		sessionStarted: time.Now(),
	}
}
```

Add the import to `model.go`'s import block: `"github.com/jonasbn/somafm-player/internal/mediakeys"`.

- [ ] **Step 5: Modify `internal/ui/model.go` — `Init`**

In `Init` (`model.go:97-111`), add the media-key wait command to the initial batch:

```go
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{waitForPlayerMsg(m.player), waitForMediaKeyCmd(m.mediaKeys), tickCmd()}
	if m.cfg.VisualizerEnabled {
		cmds = append(cmds, visualizerTickCmd())
	}
```

(the rest of `Init` is unchanged).

- [ ] **Step 6: Modify `internal/ui/model.go` — `Update` dispatch**

Right after the `visualizerTickMsg` block and before the `player.TrackChangedMsg`/etc. switch (`model.go:249-256`), add:

```go
	if _, ok := msg.(mediaKeyMsg); ok {
		return m.handleMediaKeyMsg(), waitForMediaKeyCmd(m.mediaKeys)
	}

```

- [ ] **Step 7: Modify `main.go`**

Change the imports (add `"github.com/jonasbn/somafm-player/internal/mediakeys"`) and the body:

```go
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading config:", err)
		os.Exit(1)
	}

	mk, err := mediakeys.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: media key integration unavailable:", err)
	}

	chs, fetchErr := channels.Fetch(context.Background(), channels.DefaultChannelsURL)

	m := ui.New(cfg, chs, player.NewRealPlayer(), history.New(5), mk)
```

(`mediakeys.New()` always returns a valid, usable `Controller` alongside a nil error today on every platform — the `err` check is honest handling of the constructor's signature and gives this a real fallback path if a future change ever makes registration fail.)

- [ ] **Step 8: Update the remaining `New(...)` call sites**

In `internal/ui/model_test.go`, update every existing call (lines 19, 24, 56, 220, 249) to pass a `mediakeys.NewFakeController()` as the 5th argument, e.g. line 19's `newTestModel`:

```go
func newTestModel() Model {
	chs := []channels.Channel{
		{Title: "Groove Salad"},
		{Title: "Drone Zone"},
	}
	return New(config.DefaultConfig(), chs, player.NewFakePlayer(), history.New(5), mediakeys.NewFakeController())
}
```

Apply the same pattern (append `, mediakeys.NewFakeController()`) at lines 24, 56, 220, and 249. Add the `"github.com/jonasbn/somafm-player/internal/mediakeys"` import if `newTestModelWithPlayerAndMediaKeys` from Step 1 didn't already add it.

In `internal/ui/playback_test.go:53`:

```go
	m := New(config.DefaultConfig(), []channels.Channel{{Title: "Drone Zone"}}, fp, history.New(5), mediakeys.NewFakeController())
```

Add the `mediakeys` import to `playback_test.go`.

- [ ] **Step 9: Run tests and gofmt to verify everything passes**

Run: `gofmt -l . && go build ./... && go test ./...`
Expected: `gofmt -l .` prints nothing (if it lists any file, run `gofmt -w <file>` and re-check — the struct field alignment in Step 4 in particular must match gofmt's output exactly, since `golangci-lint`'s default gofmt check runs in CI); build succeeds; all tests pass, including the two new ones from Step 1.

- [ ] **Step 10: Commit**

```bash
git add internal/ui/media_keys.go internal/ui/media_keys_test.go internal/ui/model.go internal/ui/model_test.go internal/ui/playback_test.go main.go
git commit -m "feat(ui): wire mediakeys.Controller into Model, hardware play/pause toggles mute"
```

---

## Task 5: Publish Now Playing metadata

**Files:**
- Modify: `internal/ui/media_keys.go` (add a helper)
- Modify: `internal/ui/playback.go:51-97` (`handlePlaybackMsg`)
- Modify: `internal/ui/volume.go:19-23` (`toggleMute`)
- Modify: `internal/ui/media_keys_test.go` (add coverage)

**Interfaces:**
- Consumes: `Model.mediaKeys`, `Model.nowPlaying`, `Model.cfg.Muted` (existing/Task 4 fields); `mediakeys.NowPlayingInfo`.
- Produces: `(m Model) syncNowPlaying()` — called from every playback-state-changing path.

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/media_keys_test.go`:

```go
func TestSyncNowPlaying_PublishesChannelTitleArtistAndPlayingState(t *testing.T) {
	mk := mediakeys.NewFakeController()
	m := newTestModelWithPlayerAndMediaKeys(player.NewFakePlayer(), mk)
	m.nowPlaying = nowPlayingState{
		channel:   "Groove Salad",
		title:     "Song",
		artist:    "Band",
		connected: true,
	}

	m.syncNowPlaying()

	want := mediakeys.NowPlayingInfo{Channel: "Groove Salad", Title: "Song", Artist: "Band"}
	if got := mk.NowPlaying(); got != want {
		t.Fatalf("NowPlaying() = %+v, want %+v", got, want)
	}
	if !mk.Playing() {
		t.Fatal("Playing() = false, want true (connected and not muted)")
	}
}

func TestSyncNowPlaying_NotPlayingWhenMutedOrDisconnected(t *testing.T) {
	mk := mediakeys.NewFakeController()
	m := newTestModelWithPlayerAndMediaKeys(player.NewFakePlayer(), mk)
	m.nowPlaying = nowPlayingState{channel: "Groove Salad", connected: true}
	m.cfg.Muted = true

	m.syncNowPlaying()

	if mk.Playing() {
		t.Fatal("Playing() = true while muted, want false")
	}

	m.cfg.Muted = false
	m.nowPlaying.connected = false
	m.syncNowPlaying()

	if mk.Playing() {
		t.Fatal("Playing() = true while disconnected, want false")
	}
}

func TestUpdate_MToggleMuteSyncsPlayingState(t *testing.T) {
	fp := player.NewFakePlayer()
	mk := mediakeys.NewFakeController()
	m := newTestModelWithPlayerAndMediaKeys(fp, mk)
	m.nowPlaying = nowPlayingState{channel: "Groove Salad", connected: true}

	next, _ := m.Update(key("m"))
	m = next.(Model)

	if mk.Playing() {
		t.Fatal("Playing() = true after muting, want false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/... -run 'TestSyncNowPlaying|TestUpdate_MToggleMuteSyncsPlayingState' -v`
Expected: FAIL to compile — `syncNowPlaying` undefined.

- [ ] **Step 3: Add `syncNowPlaying` to `internal/ui/media_keys.go`**

Append to the file from Task 4:

```go
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
```

- [ ] **Step 4: Call it from `internal/ui/playback.go`'s `handlePlaybackMsg`**

Add `m.syncNowPlaying()` as the last statement before each `return m, nil` in the four cases that change playback state (`streamResolvedMsg` success path, `player.TrackChangedMsg`, `player.ConnectionLostMsg`, `player.ReconnectedMsg`):

```go
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
		m.syncNowPlaying()
		return m, nil

	case player.TrackChangedMsg:
		m = m.recordCurrentTrackToHistory()
		m.nowPlaying.title = msg.Title
		m.nowPlaying.artist = msg.Artist
		m.nowPlaying.trackStarted = time.Now()
		m.syncNowPlaying()
		return m, nil

	case player.ConnectionLostMsg:
		m.nowPlaying.connected = false
		m.syncNowPlaying()
		return m, nil

	case player.ReconnectedMsg:
		m.nowPlaying.connected = true
		m.syncNowPlaying()
		return m, nil
```

(`channelsFetchedMsg` is unchanged — it doesn't touch `nowPlaying`.)

- [ ] **Step 5: Call it from `internal/ui/volume.go`'s `toggleMute`**

```go
func (m Model) toggleMute() Model {
	m.cfg.Muted = !m.cfg.Muted
	m.player.SetMuted(m.cfg.Muted)
	m.syncNowPlaying()
	return m
}
```

- [ ] **Step 6: Run tests and gofmt to verify everything passes**

Run: `gofmt -l . && go build ./... && go test ./...`
Expected: `gofmt -l .` prints nothing; build succeeds; all tests pass, including the three new ones from Step 1.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/media_keys.go internal/ui/media_keys_test.go internal/ui/playback.go internal/ui/volume.go
git commit -m "feat(ui): publish Now Playing metadata and playback state to macOS"
```

- [ ] **Step 8: Manual verification (cannot be automated — see Task 3 Step 7)**

Run through Task 3's manual checklist again on a real Mac: `go run .`, play a channel, press the hardware Play/Pause key, and this time also confirm Control Center / the menu-bar Now Playing widget / lock screen show the channel and track title, and that the displayed playback state (playing/paused icon) matches mute state.
