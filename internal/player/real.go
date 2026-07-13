package player

import (
	"context"
	"strings"
	"sync"
	"time"

	mp3 "github.com/hajimehoshi/go-mp3"
	"github.com/hajimehoshi/oto"
	"github.com/romantomjak/shoutcast"

	"github.com/jonasbn/somafm-player/internal/spectrum"
)

// backoffSchedule is the reconnect delay schedule: 1s, 2s, then 5s
// repeated indefinitely (the loop never gives up).
var backoffSchedule = []time.Duration{time.Second, 2 * time.Second, 5 * time.Second}

// otoBufferSize is the size, in bytes, of oto's internal playback buffer.
// A few multiples of the 4096-byte read chunk gives the network stream some
// slack to absorb jitter without the player starving mid-track.
const otoBufferSize = 8192

// RealPlayer streams audio from a SHOUTcast-compatible source, decodes MP3
// with go-mp3, and plays PCM through oto. It reconnects with backoff on
// connection loss and never gives up retrying.
//
// oto.NewContext maintains a single process-wide context and panics if a
// second one is created while the first is still open. RealPlayer therefore
// serializes teardown of the previous stream/decoder/context before ever
// starting a new one: Play joins on the previous run's completion (in a
// background goroutine, so callers are not blocked) before wiring up the
// next stream.
type RealPlayer struct {
	msgs   chan Msg
	mu     sync.Mutex
	volume int
	muted  bool

	cancel   context.CancelFunc
	stream   *shoutcast.Stream
	done     chan struct{}
	analyzer *spectrum.Analyzer
}

// NewRealPlayer constructs a RealPlayer ready to Play streams.
func NewRealPlayer() *RealPlayer {
	return &RealPlayer{msgs: make(chan Msg, 16), volume: 80}
}

// Messages returns the channel on which player events are delivered.
func (p *RealPlayer) Messages() <-chan Msg { return p.msgs }

// SetVolume sets the playback volume as a percentage (0-100).
func (p *RealPlayer) SetVolume(percent int) {
	p.mu.Lock()
	p.volume = percent
	p.mu.Unlock()
}

// SetMuted mutes or unmutes playback.
func (p *RealPlayer) SetMuted(muted bool) {
	p.mu.Lock()
	p.muted = muted
	p.mu.Unlock()
}

// Spectrum returns the current smoothed frequency-band values from the
// active stream's analyzer, or nil if nothing is currently playing.
func (p *RealPlayer) Spectrum() []float64 {
	p.mu.Lock()
	a := p.analyzer
	p.mu.Unlock()
	if a == nil {
		return nil
	}
	return a.Bands()
}

func (p *RealPlayer) volumeFactor() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.muted {
		return 0
	}
	return float64(p.volume) / 100.0
}

// Play starts streaming streamURL. Any previously in-flight connection is
// canceled and torn down before the new one begins; the teardown/join
// happens off the calling goroutine so Play itself does not block.
func (p *RealPlayer) Play(streamURL string) {
	p.mu.Lock()
	prevCancel := p.cancel
	prevStream := p.stream
	prevDone := p.done

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	p.cancel = cancel
	p.done = done
	p.mu.Unlock()

	go func() {
		defer close(done)

		if prevCancel != nil {
			prevCancel()
		}
		if prevStream != nil {
			prevStream.Close()
		}
		if prevDone != nil {
			// Wait for the previous run to fully release its oto
			// Context before this one tries to create a new one.
			<-prevDone
		}

		// A newer Play/Stop call may have already canceled ctx while we
		// were waiting above; run() checks ctx.Err() itself and is a
		// no-op in that case.
		p.run(ctx, streamURL)
	}()
}

// Stop cancels any in-flight connection. It does not block waiting for
// teardown to complete.
func (p *RealPlayer) Stop() {
	p.mu.Lock()
	cancel := p.cancel
	stream := p.stream
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if stream != nil {
		stream.Close()
	}
}

func (p *RealPlayer) run(ctx context.Context, streamURL string) {
	attempt := 0
	for {
		if ctx.Err() != nil {
			return
		}
		err := p.playOnce(ctx, streamURL)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			p.msgs <- ConnectionLostMsg{}
			delay := backoffSchedule[len(backoffSchedule)-1]
			if attempt < len(backoffSchedule) {
				delay = backoffSchedule[attempt]
			}
			attempt++
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
			continue
		}
		attempt = 0
	}
}

// openResult carries the outcome of an in-flight shoutcast.Open call back
// to playOnce over a channel, so the open can be raced against ctx.Done().
type openResult struct {
	stream *shoutcast.Stream
	err    error
}

func (p *RealPlayer) playOnce(ctx context.Context, streamURL string) error {
	resultCh := make(chan openResult, 1)
	go func() {
		s, err := shoutcast.Open(streamURL)
		resultCh <- openResult{stream: s, err: err}
	}()

	var stream *shoutcast.Stream
	select {
	case res := <-resultCh:
		if res.err != nil {
			return res.err
		}
		stream = res.stream
	case <-ctx.Done():
		// A newer Play/Stop call superseded us while Open was still in
		// flight. Open takes no context and can't be canceled directly,
		// so let it finish in the background and close whatever stream
		// it returns rather than leaking the connection.
		go func() {
			res := <-resultCh
			if res.stream != nil {
				res.stream.Close()
			}
		}()
		return nil
	}

	if ctx.Err() != nil {
		// Superseded while Open was in flight, but Open succeeded before
		// we noticed. Tear down the stream we just opened without
		// touching p.stream, building the decoder/oto context, or
		// sending ReconnectedMsg.
		stream.Close()
		return nil
	}

	p.mu.Lock()
	p.stream = stream
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		if p.stream == stream {
			p.stream = nil
		}
		p.mu.Unlock()
		stream.Close()
	}()

	stream.MetadataCallbackFunc = func(m *shoutcast.Metadata) {
		title, artist := splitStreamTitle(m.StreamTitle)
		p.msgs <- TrackChangedMsg{Title: title, Artist: artist}
	}

	decoder, err := mp3.NewDecoder(stream)
	if err != nil {
		return err
	}

	otoCtx, err := oto.NewContext(decoder.SampleRate(), 2, 2, otoBufferSize)
	if err != nil {
		return err
	}
	defer otoCtx.Close()

	otoPlayer := otoCtx.NewPlayer()
	defer otoPlayer.Close()

	analyzer := spectrum.New(decoder.SampleRate())
	p.mu.Lock()
	p.analyzer = analyzer
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		if p.analyzer == analyzer {
			p.analyzer = nil
		}
		p.mu.Unlock()
		analyzer.Close()
	}()

	vr := newVolumeReader(decoder, p.volumeFactor)
	p.msgs <- ReconnectedMsg{}

	buf := make([]byte, 4096)
	for {
		if ctx.Err() != nil {
			return nil
		}
		n, readErr := vr.Read(buf)
		if n > 0 {
			if _, writeErr := otoPlayer.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			analyzer.Feed(buf[:n])
		}
		if readErr != nil {
			return readErr
		}
	}
}

func splitStreamTitle(raw string) (title, artist string) {
	parts := strings.SplitN(raw, " - ", 2)
	if len(parts) == 2 {
		return parts[1], parts[0]
	}
	return raw, ""
}
