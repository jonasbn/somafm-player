//go:build !darwin

package mediakeys

// New returns a Controller that does nothing: no hardware key events are
// ever delivered, and SetNowPlaying/SetPlaying/Close are no-ops. macOS
// media-key integration only exists on darwin; every other OS gets this
// stub so the rest of the module stays portable.
func New() (Controller, error) {
	return &noopController{events: make(chan Event)}, nil
}

type noopController struct {
	events chan Event
}

func (c *noopController) Events() <-chan Event           { return c.events }
func (c *noopController) SetNowPlaying(NowPlayingInfo)    {}
func (c *noopController) SetPlaying(bool)                 {}
func (c *noopController) Close()                          {}
