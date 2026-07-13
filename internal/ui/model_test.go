package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdate_QuitsOnQ(t *testing.T) {
	m := New()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	if cmd == nil {
		t.Fatal("expected a command to be returned for quit key")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}
