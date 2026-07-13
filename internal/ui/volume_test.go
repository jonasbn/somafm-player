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
