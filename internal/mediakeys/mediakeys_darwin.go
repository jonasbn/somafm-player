//go:build darwin

package mediakeys

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework MediaPlayer -framework Foundation
#include "bridge_darwin.h"
#include <stdlib.h>
*/
import "C"

import (
	"runtime"
	"unsafe"
)

// activeEvents is the single darwin Controller's event channel.
// mediakeys.New is only ever called once per process (from main.go), so
// a package-level channel is simpler than threading a context pointer
// through the cgo callback boundary.
var activeEvents = make(chan Event, 1)

//export goMediaKeyPlayPause
func goMediaKeyPlayPause() {
	// Non-blocking send: this runs on the ObjC run-loop thread inside a
	// command handler callback and must never block waiting for Go to
	// catch up, mirroring internal/spectrum.Analyzer.Feed's send.
	select {
	case activeEvents <- PlayPauseEvent:
	default:
	}
}

type darwinController struct {
	events chan Event
}

// New registers this process as a Now Playing target and starts the
// background run-loop goroutine that keeps the registration alive.
func New() (Controller, error) {
	go func() {
		runtime.LockOSThread()
		C.mediakeys_start()
		C.mediakeys_run_loop() // never returns
	}()

	return &darwinController{events: activeEvents}, nil
}

func (c *darwinController) Events() <-chan Event {
	return c.events
}

func (c *darwinController) SetNowPlaying(info NowPlayingInfo) {
	cChannel := C.CString(info.Channel)
	cTitle := C.CString(info.Title)
	cArtist := C.CString(info.Artist)
	defer C.free(unsafe.Pointer(cChannel))
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cArtist))

	C.mediakeys_set_now_playing(cChannel, cTitle, cArtist)
}

func (c *darwinController) SetPlaying(playing bool) {
	v := C.int(0)
	if playing {
		v = C.int(1)
	}
	C.mediakeys_set_playing(v)
}

func (c *darwinController) Close() {
	C.mediakeys_stop()
}
