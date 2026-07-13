package ui

import "testing"

func TestUpdate_VKeyTogglesVisualizerAndSchedulesTickWhenEnabling(t *testing.T) {
	m := newTestModel()
	m.cfg.VisualizerEnabled = false

	next, cmd := m.Update(key("v"))
	m = next.(Model)

	if !m.cfg.VisualizerEnabled {
		t.Fatal("VisualizerEnabled = false after v, want true")
	}
	if cmd == nil {
		t.Fatal("expected a tick-scheduling cmd when enabling the visualizer")
	}
}

func TestUpdate_VKeyDisablingReturnsNoCmd(t *testing.T) {
	m := newTestModel()
	m.cfg.VisualizerEnabled = true

	next, cmd := m.Update(key("v"))
	m = next.(Model)

	if m.cfg.VisualizerEnabled {
		t.Fatal("VisualizerEnabled = true after v, want false")
	}
	if cmd != nil {
		t.Fatal("expected no cmd when disabling the visualizer")
	}
}
