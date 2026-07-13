package ui

import (
	"testing"
	"time"

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

func TestUpdate_EnterOnBookmarkedChannelResolvesAndPlays(t *testing.T) {
	m := newTestModel()
	m.cfg.BookmarkedChannels = []string{"Drone Zone"}
	m.mode = viewBookmarkedChannels
	m.selected = 0

	_, cmd := m.Update(key("enter"))
	if cmd == nil {
		t.Fatal("expected enter to return a resolve command")
	}
	msg := cmd()
	resolved, ok := msg.(streamResolvedMsg)
	if !ok {
		t.Fatalf("expected streamResolvedMsg, got %T", msg)
	}
	if resolved.channelTitle != "Drone Zone" {
		t.Fatalf("resolved.channelTitle = %q, want Drone Zone", resolved.channelTitle)
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

func TestRecordCurrentTrackToHistory_SetsPlayedAtToNow(t *testing.T) {
	m := newTestModel()
	m.nowPlaying = nowPlayingState{title: "Track", artist: "Artist", channel: "Channel"}

	before := time.Now()
	m = m.recordCurrentTrackToHistory()
	after := time.Now()

	entries := m.hist.Entries()
	if len(entries) != 1 {
		t.Fatalf("history entries = %+v, want 1 entry", entries)
	}
	playedAt := entries[0].PlayedAt
	if playedAt.IsZero() {
		t.Fatal("recorded entry PlayedAt is zero value, want time.Now() at record time")
	}
	if playedAt.Before(before) || playedAt.After(after) {
		t.Fatalf("recorded entry PlayedAt = %v, want between %v and %v", playedAt, before, after)
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
