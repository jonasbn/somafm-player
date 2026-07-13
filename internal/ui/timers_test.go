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

func TestHandleVisualizerTick_PullsBandsFromPlayer(t *testing.T) {
	m := newTestModel() // FakePlayer.Spectrum() always returns nil
	m.bands = []float64{0.9, 0.9}

	m = m.handleVisualizerTick()

	if m.bands != nil {
		t.Fatalf("bands = %v, want nil (pulled from FakePlayer.Spectrum())", m.bands)
	}
}
