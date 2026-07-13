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
