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
