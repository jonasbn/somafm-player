package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jonasbn/somafm-player/internal/channels"
	"github.com/jonasbn/somafm-player/internal/config"
	"github.com/jonasbn/somafm-player/internal/history"
	"github.com/jonasbn/somafm-player/internal/player"
	"github.com/jonasbn/somafm-player/internal/ui"
)

const channelsURL = "https://somafm.com/channels.json"

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading config:", err)
		os.Exit(1)
	}

	chs, err := channels.Fetch(context.Background(), channelsURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error fetching channel list:", err)
		os.Exit(1)
	}

	m := ui.New(cfg, chs, player.NewRealPlayer(), history.New(5))
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
