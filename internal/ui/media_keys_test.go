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
