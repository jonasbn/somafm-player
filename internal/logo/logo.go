// Package logo provides the ASCII-art banner shown above the player,
// selecting a channel-specific variant when one is currently playing.
package logo

// defaultArt is the "somafm" banner (patorjk.com "Big" font), shown when
// no channel-specific art matches.
var defaultArt = []string{
	"                               __           ",
	"                              / _|          ",
	"  ___  ___  _ __ ___   __ _  | |_ _ __ ___  ",
	" / __|/ _ \\| '_ ` _ \\ / _` | |  _| '_ ` _ \\ ",
	" \\__ \\ (_) | | | | | | (_| | | | | | | | | |",
	" |___/\\___/|_| |_| |_|\\__,_| |_| |_| |_| |_|",
	"                                            ",
}

// droneZoneArt is shown for the "Drone Zone" and "Drone Zone 2" channels.
var droneZoneArt = []string{
	"  ____  ____   ___  _   _ _____   ________  _   _ _____ ",
	" |  _ \\|  _ \\ / _ \\| \\ | | ____| |__  / _ \\| \\ | | ____|",
	" | | | | |_) | | | |  \\| |  _|     / / | | |  \\| |  _|  ",
	" | |_| |  _ <| |_| | |\\  | |___   / /| |_| | |\\  | |___ ",
	" |____/|_| \\_\\\\___/|_| \\_|_____| /____\\___/|_| \\_|_____|",
	"                                                        ",
}

// deepSpaceOneArt is shown for the "Deep Space One" channel.
var deepSpaceOneArt = []string{
	" ____                 _____                    _____         ",
	"|    \\ ___ ___ ___   |   __|___ ___ ___ ___   |     |___ ___ ",
	"|  |  | -_| -_| . |  |__   | . | .'|  _| -_|  |  |  |   | -_|",
	"|____/|___|___|  _|  |_____|  _|__,|___|___|  |_____|_|_|___|",
	"              |_|          |_|",
}

var byChannel = map[string][]string{
	"Drone Zone":     droneZoneArt,
	"Drone Zone 2":   droneZoneArt,
	"Deep Space One": deepSpaceOneArt,
}

// For returns the ASCII art lines for channelTitle, falling back to the
// default "somafm" art for any unmatched title (including ""). isDefault
// reports whether the fallback was used.
func For(channelTitle string) (lines []string, isDefault bool) {
	if art, ok := byChannel[channelTitle]; ok {
		return art, false
	}
	return defaultArt, true
}

// Width returns the length of the widest line in lines. All art is pure
// ASCII, so byte length equals display width.
func Width(lines []string) int {
	max := 0
	for _, l := range lines {
		if len(l) > max {
			max = len(l)
		}
	}
	return max
}
