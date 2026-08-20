// Package mediakeys integrates somafm-player with macOS hardware media
// keys (play/pause) via MPRemoteCommandCenter/MPNowPlayingInfoCenter. The
// darwin implementation lives in mediakeys_darwin.go; mediakeys_other.go
// provides a no-op stub for every other OS.
package mediakeys

// Event identifies a media-key action delivered by a Controller.
type Event int

// PlayPauseEvent is sent when the hardware play, pause, or toggle
// play/pause key is pressed. somafm-player has no true "pause" for a
// live stream, so all three map to the same event and the UI layer
// treats it as a mute toggle.
const PlayPauseEvent Event = iota

// NowPlayingInfo is the metadata published to macOS so Control Center,
// the menu-bar Now Playing widget, and the lock screen can display it.
type NowPlayingInfo struct {
	Channel string
	Title   string
	Artist  string
}

// Controller receives hardware media-key events and publishes playback
// state to macOS. Callers must call Close when done.
type Controller interface {
	// Events delivers a PlayPauseEvent each time the hardware key is
	// pressed. The channel is never closed by the Controller.
	Events() <-chan Event
	// SetNowPlaying updates the metadata shown by macOS. Empty fields
	// are treated as "unknown" and omitted from the display.
	SetNowPlaying(info NowPlayingInfo)
	// SetPlaying updates the playback state shown by macOS.
	SetPlaying(playing bool)
	// Close unregisters the controller and releases its resources.
	Close()
}

// NewNoop returns a Controller that does nothing: no hardware key events
// are ever delivered, and SetNowPlaying/SetPlaying/Close are no-ops. It's
// the stub used on every non-darwin platform, and the fallback callers
// should use if New ever returns a non-nil error, so a nil Controller
// interface never reaches the rest of the program.
func NewNoop() Controller {
	return &noopController{events: make(chan Event)}
}

type noopController struct {
	events chan Event
}

func (c *noopController) Events() <-chan Event         { return c.events }
func (c *noopController) SetNowPlaying(NowPlayingInfo) {}
func (c *noopController) SetPlaying(bool)              {}
func (c *noopController) Close()                       {}
