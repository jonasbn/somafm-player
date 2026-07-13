package player

import "testing"

func TestRealPlayer_SpectrumReturnsNilWhenNoStreamActive(t *testing.T) {
	p := NewRealPlayer()
	if got := p.Spectrum(); got != nil {
		t.Fatalf("Spectrum() = %v, want nil when no stream is playing", got)
	}
}
