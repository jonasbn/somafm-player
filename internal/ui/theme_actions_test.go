package ui

import "testing"

func TestUpdate_TKeyCyclesTheme(t *testing.T) {
	m := newTestModel()
	m.cfg.Theme = "Nord"

	next, _ := m.Update(key("t"))
	m = next.(Model)
	if m.cfg.Theme != "Dracula" {
		t.Fatalf("theme after t = %q, want Dracula", m.cfg.Theme)
	}
}
