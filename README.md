# SomaFM Drone Zone Player

This should scope out a minimal Go program to play SomaFM's Drone Zone stream.

On the codec details, SomaFM's FAQ explains: they offer 128k MP3, 128k AAC (aka AAC-LC, part of the MPEG4 standard), and 64 and 32kb AAC-HE (aka aacPlus) streams, with the 128k AAC streams sounding best, followed closely by the 64k AAC-HE streams, then 128k MP3.

Transport-wise, these are **Icecast** streams — SomaFM uses Icecast KH servers — so each `.pls` file just points to a raw Icecast mount (HTTP stream of continuous MP3 or ADTS/AAC frames with ICY metadata headers for song titles), not something like HLS or DASH.

So concretely, if you fetch `https://somafm.com/dronezone.pls`, the `File1=` line resolves to an MP3 or AAC Icecast stream URL like `https://ice5.somafm.com/dronezone-128-mp3` — plain HTTP audio streaming, no segmented playlist/manifest involved once you're past the `.pls`.

Since these `.pls` files just point to plain **Icecast** HTTP streams (not HLS/DASH), the useful Go libraries shift away from playlist parsing and toward: (1) reading ICY metadata for "now playing" info, and (2) decoding/playing the actual audio codec.This changes the picture quite a bit. Since it's a raw Icecast/Shoutcast stream (mp3 or AAC), not a segmented format like HLS, the useful Go tooling splits into three layers:

**1. Reading the ICY (Shoutcast/Icecast) metadata protocol**
`github.com/romantomjak/shoutcast` is exactly built for this — it opens an HTTP connection with the `Icy-Metadata: 1` header, parses out the interleaved metadata blocks (using `icy-metaint`), and gives you both a clean audio stream and a callback for `StreamTitle` (the "now playing" info). Package docs show it directly wired to a decoder+player: it opens the stream via `shoutcast.Open(url)`, registers `stream.MetadataCallbackFunc` to print the currently playing track, then pipes the cleaned stream straight into an MP3 decoder and audio player.

**2. Decoding the audio codec**

- For the MP3 variants (`dronezone-128-mp3`, `dronezone-256-mp3`): `github.com/hajimehoshi/go-mp3` is the standard pure-Go MP3 decoder, and it's literally what the shoutcast example above uses.
- For the AAC/AAC-HE variants (`dronezone-128-aac`, `-64-aac`, `-32-aac`): this is the weak spot in the Go ecosystem — there's no mature pure-Go AAC decoder. Practically, people either shell out to `ffmpeg`/`faad2` as a subprocess, or just stick to the MP3 stream URLs from SomaFM to stay pure-Go.

**3. Playback (if you want to actually hear it, not just process it)**
`github.com/hajimehoshi/oto` is the typical pairing — a cross-platform low-level audio output library that takes PCM samples from the decoder and plays them.

So a minimal pure-Go pipeline for Drone Zone would be: parse `.pls` (trivial, as established) → `shoutcast.Open()` on the `-mp3` URL → `go-mp3` decoder → `oto` player, with the metadata callback giving you track titles for free. If you want AAC specifically, you'd need to bring in an external decoder binary since Go's own codec support stops at MP3.

## Features

The player should be a simple Go program that:

- Is a TUI (terminal user interface) that shows the currently playing track and allows you to quit with a keypress.
- It should display the current track title, and optionally the artist if available.
- It should handle reconnections gracefully if the stream drops.
- It should display the stream bitrate and codec type (MP3 or AAC) if possible.
- It should display the stream, so when we extend the player with more SomaFM channels, we can show a list of available channels and allow the user to switch between them.
- The user should be able to bookmark favorite channels for quick access.
- The user should be able to adjust the volume of the playback.
- The user should be able to mute/unmute the playback.
- The user should be able to see the current playback time and total duration of the track if available.
- The user can bookmark tunes that they like and view a list of their bookmarked tunes.
- The user can view the history of previously played tracks and see the time they were played. Remembering the last 5 tracks, if the application is closed and reopened, the history does not need to be available, but it should be available during the current session.

- The player should be themeable, allowing users to customize the colors and appearance of the interface.
- Themes: "Nord", "Dracula", "Gruvbox", "Tokyo Night", "Solarized Dark", "Solarized Light"

## Stack

- Go
- A TUI framework (e.g., `tview` or `bubbletea`) for the terminal interface.

## Usage

```
go run .
```

| Key | Action |
|---|---|
| `tab` | toggle focus between Now Playing and the list panel |
| `j`/`k` or arrows | move selection |
| `enter` | play selected channel |
| `c` / `f` / `s` / `H` | switch list panel: Channels / Bookmarked Channels / Bookmarked Tunes / History |
| `b` | bookmark (context-sensitive: tune on Now Playing, channel on Channels/Bookmarked Channels, tune on History) |
| `+`/`-` or arrows | volume up/down |
| `m` | mute/unmute |
| `t` | cycle theme |
| `v` | toggle equalizer visualizer |
| `r` | retry fetching the channel list (e.g. after a startup network error) |
| `q` | quit |

Config, including bookmarks, volume, theme, last-played channel, and visualizer setting, is stored at
`~/.config/somafm-player/config.json` (or under `$XDG_CONFIG_HOME` if set).
