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
