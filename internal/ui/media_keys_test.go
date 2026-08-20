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
