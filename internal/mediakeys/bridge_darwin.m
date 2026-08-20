#import "bridge_darwin.h"
#import <Foundation/Foundation.h>
#import <MediaPlayer/MediaPlayer.h>
#import "_cgo_export.h"

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

    [center.playCommand addTargetWithHandler:handler];
    [center.pauseCommand addTargetWithHandler:handler];
    [center.togglePlayPauseCommand addTargetWithHandler:handler];

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
    while (1) {
        [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode
                                  beforeDate:[NSDate distantFuture]];
    }
}

void mediakeys_stop(void) {
    MPRemoteCommandCenter *center = [MPRemoteCommandCenter sharedCommandCenter];
    [center.playCommand removeTarget:nil];
    [center.pauseCommand removeTarget:nil];
    [center.togglePlayPauseCommand removeTarget:nil];
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
