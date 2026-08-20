package mediakeys

import "sync"

// FakeController is a Controller test double, mirroring
// player.FakePlayer: tests use Emit to simulate a hardware key press,
// and NowPlaying/Playing/Closed to assert what the code under test
// published.
type FakeController struct {
	mu         sync.Mutex
	events     chan Event
	nowPlaying NowPlayingInfo
	playing    bool
	closed     bool
	syncCalls  int
}

func NewFakeController() *FakeController {
	return &FakeController{events: make(chan Event, 4)}
}

func (f *FakeController) Events() <-chan Event {
	return f.events
}

func (f *FakeController) SetNowPlaying(info NowPlayingInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nowPlaying = info
	f.syncCalls++
}

func (f *FakeController) SetPlaying(playing bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.playing = playing
}

func (f *FakeController) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
}

// Emit simulates a hardware media-key press, delivering e on Events().
func (f *FakeController) Emit(e Event) {
	f.events <- e
}

func (f *FakeController) NowPlaying() NowPlayingInfo {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.nowPlaying
}

func (f *FakeController) Playing() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.playing
}

func (f *FakeController) Closed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

// SyncCalls returns how many times SetNowPlaying was called, so tests can
// assert a publish did or didn't happen without relying on the published
// value alone (an unpublished zero value and a published empty one are
// otherwise indistinguishable).
func (f *FakeController) SyncCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.syncCalls
}
