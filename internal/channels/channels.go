package channels

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type Playlist struct {
	URL     string `json:"url"`
	Format  string `json:"format"`
	Quality string `json:"quality"`
}

type Channel struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Genre       string     `json:"genre"`
	Playlists   []Playlist `json:"playlists"`
}

type channelsResponse struct {
	Channels []Channel `json:"channels"`
}

func Parse(data []byte) ([]Channel, error) {
	var resp channelsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Channels, nil
}

func Fetch(ctx context.Context, url string) ([]Channel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

func qualityRank(q string) int {
	switch q {
	case "highest":
		return 3
	case "high":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func (c Channel) BestMP3Stream() string {
	var best Playlist
	found := false
	for _, p := range c.Playlists {
		if p.Format != "mp3" {
			continue
		}
		if !found || qualityRank(p.Quality) > qualityRank(best.Quality) {
			best = p
			found = true
		}
	}
	if !found {
		return ""
	}
	return best.URL
}

func ResolveStreamURL(ctx context.Context, plsURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, plsURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d from %s", resp.StatusCode, plsURL)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "File1=") {
			return strings.TrimPrefix(line, "File1="), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no File1 entry found in %s", plsURL)
}

var bitratePattern = regexp.MustCompile(`-(\d+)-(mp3|aac)`)

func ParseBitrateFromURL(url string) (bitrate int, codec string) {
	m := bitratePattern.FindStringSubmatch(url)
	if m == nil {
		return 0, ""
	}
	bitrate, _ = strconv.Atoi(m[1])
	return bitrate, strings.ToUpper(m[2])
}
