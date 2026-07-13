package ui

import tea "github.com/charmbracelet/bubbletea"

// toggleVisualizer flips the visualizer on/off. Turning it on kicks off
// the fast visualizer tick loop (Init only schedules that loop once, at
// startup, based on the config value at that time); turning it off simply
// stops rescheduling — handled by the visualizerTickMsg case in Update.
func (m Model) toggleVisualizer() (Model, tea.Cmd) {
	m.cfg.VisualizerEnabled = !m.cfg.VisualizerEnabled
	if m.cfg.VisualizerEnabled {
		return m, visualizerTickCmd()
	}
	return m, nil
}
