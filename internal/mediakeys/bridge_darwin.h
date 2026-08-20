#ifndef SOMAFM_MEDIAKEYS_BRIDGE_DARWIN_H
#define SOMAFM_MEDIAKEYS_BRIDGE_DARWIN_H

// mediakeys_start registers this process as a Now Playing target: it
// installs handlers on MPRemoteCommandCenter's play/pause/toggle
// commands that call back into Go via goMediaKeyPlayPause (see
// mediakeys_darwin.go's //export directive and the generated
// _cgo_export.h).
void mediakeys_start(void);

// mediakeys_run_loop pumps the current thread's run loop forever. The
// command handlers registered by mediakeys_start only fire while a run
// loop is being pumped, so this must run on its own goroutine for the
// lifetime of the process (see mediakeys_darwin.go's New).
void mediakeys_run_loop(void);

// mediakeys_stop unregisters the command handlers installed by
// mediakeys_start.
void mediakeys_stop(void);

// mediakeys_set_now_playing publishes channel/title/artist to
// MPNowPlayingInfoCenter. title and artist may be empty strings, in
// which case they're omitted (channel is used as the displayed title).
void mediakeys_set_now_playing(const char *channel, const char *title, const char *artist);

// mediakeys_set_playing updates the playback state shown by macOS.
// playing is a C bool encoded as 0/1.
void mediakeys_set_playing(int playing);

#endif
