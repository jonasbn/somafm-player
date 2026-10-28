package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig_VisualizerDisabledByDefault(t *testing.T) {
	if DefaultConfig().VisualizerEnabled {
		t.Fatal("DefaultConfig().VisualizerEnabled = true, want false (opt-in feature)")
	}
}

func TestSaveThenLoad_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := Config{
		LastChannel:        "Drone Zone",
		Volume:             65,
		Muted:              false,
		Theme:              "Dracula",
		BookmarkedChannels: []string{"Drone Zone", "Groove Salad"},
		BookmarkedTunes: []BookmarkedTune{
			{Title: "Track", Artist: "Artist", Channel: "Drone Zone", BookmarkedAt: time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)},
		},
		VisualizerEnabled: true,
		VisualizerStyle:   "classic",
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
	if loaded.VisualizerEnabled != cfg.VisualizerEnabled {
		t.Fatalf("loaded.VisualizerEnabled = %v, want %v", loaded.VisualizerEnabled, cfg.VisualizerEnabled)
	}
	if loaded.VisualizerStyle != cfg.VisualizerStyle {
		t.Fatalf("loaded.VisualizerStyle = %q, want %q", loaded.VisualizerStyle, cfg.VisualizerStyle)
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

func TestLoad_PartialFileKeepsDefaultsForMissingFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path, err := Path()
	if err != nil {
		t.Fatalf("Path returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	// A hand-written config with only some keys, e.g. copied from the
	// README, must not zero out the rest (volume 0 makes playback silent).
	partial := []byte(`{"visualizerEnabled": true, "visualizerStyle": "classic"}`)
	if err := os.WriteFile(path, partial, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	def := DefaultConfig()
	if cfg.Volume != def.Volume || cfg.Theme != def.Theme {
		t.Fatalf("Load() = volume %d, theme %q; want defaults volume %d, theme %q", cfg.Volume, cfg.Theme, def.Volume, def.Theme)
	}
	if !cfg.VisualizerEnabled || cfg.VisualizerStyle != "classic" {
		t.Fatalf("Load() = %+v, want the file's visualizer settings applied", cfg)
	}
}

func TestLoad_ExplicitZeroVolumeIsKept(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := DefaultConfig()
	cfg.Volume = 0
	if err := Save(cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Volume != 0 {
		t.Fatalf("Load().Volume = %d, want 0 (explicitly saved)", loaded.Volume)
	}
}
