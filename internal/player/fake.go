package player

import "sync"

type FakePlayer struct {
	mu         sync.Mutex
	msgs       chan Msg
	playedURLs []string
	volume     int
	muted      bool
	stopped    bool
}

func NewFakePlayer() *FakePlayer {
	return &FakePlayer{msgs: make(chan Msg, 16), volume: 80}
}

func (p *FakePlayer) Play(streamURL string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.playedURLs = append(p.playedURLs, streamURL)
	p.stopped = false
}

func (p *FakePlayer) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopped = true
}

func (p *FakePlayer) SetVolume(percent int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.volume = percent
}

func (p *FakePlayer) SetMuted(muted bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.muted = muted
}

func (p *FakePlayer) Messages() <-chan Msg {
	return p.msgs
}

// Spectrum always returns nil: FakePlayer does no real decoding, so there
// is no audio to analyze. UI code consuming this exercises the nil
// ("nothing playing" / flat bars) path deterministically in tests.
func (p *FakePlayer) Spectrum() []float64 {
	return nil
}

func (p *FakePlayer) Emit(msg Msg) {
	p.msgs <- msg
}

func (p *FakePlayer) PlayedURLs() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.playedURLs...)
}

func (p *FakePlayer) Volume() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volume
}

func (p *FakePlayer) Muted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.muted
}

func (p *FakePlayer) Stopped() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stopped
}
