package player

type Msg interface{}

type TrackChangedMsg struct {
	Title  string
	Artist string
}

type ConnectionLostMsg struct{}

type ReconnectedMsg struct{}

type Player interface {
	Play(streamURL string)
	Stop()
	SetVolume(percent int)
	SetMuted(muted bool)
	Messages() <-chan Msg
}
