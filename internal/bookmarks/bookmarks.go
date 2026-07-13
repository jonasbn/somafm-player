package bookmarks

import "github.com/jonasbn/somafm-player/internal/config"

func ToggleChannel(cfg *config.Config, channelTitle string) {
	for i, c := range cfg.BookmarkedChannels {
		if c == channelTitle {
			cfg.BookmarkedChannels = append(cfg.BookmarkedChannels[:i], cfg.BookmarkedChannels[i+1:]...)
			return
		}
	}
	cfg.BookmarkedChannels = append(cfg.BookmarkedChannels, channelTitle)
}

func IsChannelBookmarked(cfg config.Config, channelTitle string) bool {
	for _, c := range cfg.BookmarkedChannels {
		if c == channelTitle {
			return true
		}
	}
	return false
}

func AddTune(cfg *config.Config, tune config.BookmarkedTune) {
	for _, t := range cfg.BookmarkedTunes {
		if t.Title == tune.Title && t.Artist == tune.Artist && t.Channel == tune.Channel {
			return
		}
	}
	cfg.BookmarkedTunes = append(cfg.BookmarkedTunes, tune)
}
