#import "bridge_darwin.h"
#import <Foundation/Foundation.h>
#import <MediaPlayer/MediaPlayer.h>
#import "_cgo_export.h"

// addTargetWithHandler: returns an opaque token identifying the
// registration; removeTarget: needs that same token to unregister it
// (removeTarget:nil matches nothing and is a silent no-op). Stash the
// three tokens here so mediakeys_stop can actually undo mediakeys_start.
static id playTarget = nil;
static id pauseTarget = nil;
static id toggleTarget = nil;

void mediakeys_start(void) {
    MPRemoteCommandCenter *center = [MPRemoteCommandCenter sharedCommandCenter];

    // This SDK does not define an "MPRemoteCommandHandler" typedef alias
    // for the handler block type (only MPRemoteCommandHandlerStatus, the
    // enum it returns) -- addTargetWithHandler: takes the block type
    // spelled out below, so that's what we declare here.
    MPRemoteCommandHandlerStatus (^handler)(MPRemoteCommandEvent *event) = ^MPRemoteCommandHandlerStatus(MPRemoteCommandEvent *event) {
        goMediaKeyPlayPause();
        return MPRemoteCommandHandlerStatusSuccess;
    };

    playTarget = [center.playCommand addTargetWithHandler:handler];
    pauseTarget = [center.pauseCommand addTargetWithHandler:handler];
    toggleTarget = [center.togglePlayPauseCommand addTargetWithHandler:handler];

    center.playCommand.enabled = YES;
    center.pauseCommand.enabled = YES;
    center.togglePlayPauseCommand.enabled = YES;
}

void mediakeys_run_loop(void) {
    // [[NSRunLoop currentRunLoop] run] returns as soon as the loop has no
    // scheduled input sources, which can happen depending on how the
    // command center wires up its internal XPC connection. Looping
    // runMode:beforeDate: with a distant-future date is the standard
    // idiom for pumping a background thread's run loop forever.
    //
    // This thread has no NSApplication wrapping each iteration in its
    // own autorelease pool the way the main thread would, so we do it
    // ourselves -- otherwise autoreleased objects created while
    // servicing the run loop (e.g. by command-handler block invocation)
    // never get released. runLoop/distantFuture are hoisted out of the
    // loop since both are effectively singletons; no need to refetch
    // them every iteration.
    NSRunLoop *runLoop = [NSRunLoop currentRunLoop];
    NSDate *distantFuture = [NSDate distantFuture];
    while (1) {
        @autoreleasepool {
            [runLoop runMode:NSDefaultRunLoopMode beforeDate:distantFuture];
        }
    }
}

void mediakeys_stop(void) {
    MPRemoteCommandCenter *center = [MPRemoteCommandCenter sharedCommandCenter];
    [center.playCommand removeTarget:playTarget];
    [center.pauseCommand removeTarget:pauseTarget];
    [center.togglePlayPauseCommand removeTarget:toggleTarget];
    playTarget = nil;
    pauseTarget = nil;
    toggleTarget = nil;
}

void mediakeys_set_now_playing(const char *channel, const char *title, const char *artist) {
    NSMutableDictionary *info = [NSMutableDictionary dictionary];

    NSString *channelStr = [NSString stringWithUTF8String:channel];
    NSString *titleStr = (title != NULL && title[0] != '\0') ? [NSString stringWithUTF8String:title] : nil;
    NSString *artistStr = (artist != NULL && artist[0] != '\0') ? [NSString stringWithUTF8String:artist] : nil;

    info[MPMediaItemPropertyTitle] = titleStr ?: channelStr;
    if (artistStr != nil) {
        info[MPMediaItemPropertyArtist] = artistStr;
    }

    [MPNowPlayingInfoCenter defaultCenter].nowPlayingInfo = info;
}

void mediakeys_set_playing(int playing) {
    [MPNowPlayingInfoCenter defaultCenter].playbackState =
        playing ? MPNowPlayingPlaybackStatePlaying : MPNowPlayingPlaybackStatePaused;
}
