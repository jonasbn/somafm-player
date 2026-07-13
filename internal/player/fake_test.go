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

func TestFakePlayer_SpectrumReturnsNil(t *testing.T) {
	p := NewFakePlayer()
	if got := p.Spectrum(); got != nil {
		t.Fatalf("Spectrum() = %v, want nil", got)
	}
}
