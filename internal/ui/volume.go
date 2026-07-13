package ui

func clampVolume(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func (m Model) adjustVolume(delta int) Model {
	m.cfg.Volume = clampVolume(m.cfg.Volume + delta)
	m.player.SetVolume(m.cfg.Volume)
	return m
}

func (m Model) toggleMute() Model {
	m.cfg.Muted = !m.cfg.Muted
	m.player.SetMuted(m.cfg.Muted)
	return m
}
