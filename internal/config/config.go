package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type BookmarkedTune struct {
	Title        string    `json:"title"`
	Artist       string    `json:"artist"`
	Channel      string    `json:"channel"`
	BookmarkedAt time.Time `json:"bookmarkedAt"`
}

type Config struct {
	LastChannel        string           `json:"lastChannel"`
	Volume             int              `json:"volume"`
	Muted              bool             `json:"muted"`
	Theme              string           `json:"theme"`
	BookmarkedChannels []string         `json:"bookmarkedChannels"`
	BookmarkedTunes    []BookmarkedTune `json:"bookmarkedTunes"`
}

func DefaultConfig() Config {
	return Config{Volume: 80, Theme: "Nord"}
}

func Path() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "somafm-player", "config.json"), nil
}

func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
