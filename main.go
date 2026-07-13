package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jonasbn/somafm-player/internal/channels"
	"github.com/jonasbn/somafm-player/internal/config"
	"github.com/jonasbn/somafm-player/internal/history"
	"github.com/jonasbn/somafm-player/internal/player"
	"github.com/jonasbn/somafm-player/internal/ui"
)

func main() {
	// github.com/romantomjak/shoutcast logs via the standard "log" package
	// (connection open/close, HTTP headers, metadata updates) directly to
	// stderr, which corrupts the Bubble Tea alternate-screen display since
	// that stream isn't managed by the TUI. Silence it here, before the
	// audio pipeline can ever open a connection.
	log.SetOutput(io.Discard)

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading config:", err)
		os.Exit(1)
	}

	chs, fetchErr := channels.Fetch(context.Background(), channels.DefaultChannelsURL)

	m := ui.New(cfg, chs, player.NewRealPlayer(), history.New(5))
	if fetchErr != nil {
		m = m.WithStartupError(fmt.Sprintf("Couldn't load channel list — check your connection, press r to retry (%v)", fetchErr))
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
