package channels

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const sampleChannelsJSON = `{
  "channels": [
    {
      "id": "dronezone",
      "title": "Drone Zone",
      "description": "Served best chilled, safe with most food groups.",
      "genre": "ambient",
      "playlists": [
        {"url": "http://example.test/dronezone.pls", "format": "mp3", "quality": "highest"},
        {"url": "http://example.test/dronezonelo.pls", "format": "mp3", "quality": "low"},
        {"url": "http://example.test/dronezone-aac.pls", "format": "aac", "quality": "highest"}
      ]
    }
  ]
}`

const samplePLS = "[playlist]\nNumberOfEntries=1\nFile1=https://ice5.somafm.com/dronezone-128-mp3\nTitle1=SomaFM: Drone Zone\nLength1=-1\nVersion=2\n"

const malformedPLS = "[playlist]\nNumberOfEntries=0\nVersion=2\n"

const aacOnlyChannelJSON = `{
  "channels": [
    {
      "id": "dronezone",
      "title": "Drone Zone",
      "description": "Served best chilled, safe with most food groups.",
      "genre": "ambient",
      "playlists": [
        {"url": "http://example.test/dronezone-aac.pls", "format": "aac", "quality": "highest"}
      ]
    }
  ]
}`

func TestParse_ExtractsChannels(t *testing.T) {
	chs, err := Parse([]byte(sampleChannelsJSON))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(chs) != 1 {
		t.Fatalf("Parse returned %d channels, want 1", len(chs))
	}
	if chs[0].Title != "Drone Zone" || chs[0].Genre != "ambient" {
		t.Fatalf("unexpected channel: %+v", chs[0])
	}
}

func TestBestMP3Stream_PicksHighestQualityMP3(t *testing.T) {
	chs, err := Parse([]byte(sampleChannelsJSON))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	got := chs[0].BestMP3Stream()
	want := "http://example.test/dronezone.pls"
	if got != want {
		t.Fatalf("BestMP3Stream() = %q, want %q", got, want)
	}
}

func TestBestMP3Stream_NoMP3VariantReturnsEmptyString(t *testing.T) {
	chs, err := Parse([]byte(aacOnlyChannelJSON))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	got := chs[0].BestMP3Stream()
	if got != "" {
		t.Fatalf("BestMP3Stream() = %q, want empty string", got)
	}
}

func TestFetch_ParsesFromHTTPServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(sampleChannelsJSON))
	}))
	defer srv.Close()

	chs, err := Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(chs) != 1 || chs[0].Title != "Drone Zone" {
		t.Fatalf("unexpected fetch result: %+v", chs)
	}
}

func TestResolveStreamURL_ParsesFile1FromPLS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(samplePLS))
	}))
	defer srv.Close()

	got, err := ResolveStreamURL(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("ResolveStreamURL returned error: %v", err)
	}
	want := "https://ice5.somafm.com/dronezone-128-mp3"
	if got != want {
		t.Fatalf("ResolveStreamURL() = %q, want %q", got, want)
	}
}

func TestFetch_NonOKStatusReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(sampleChannelsJSON))
	}))
	defer srv.Close()

	_, err := Fetch(context.Background(), srv.URL)
	if err == nil {
		t.Fatalf("Fetch returned nil error, want non-nil for non-OK status")
	}
}

func TestResolveStreamURL_MalformedPLSReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(malformedPLS))
	}))
	defer srv.Close()

	_, err := ResolveStreamURL(context.Background(), srv.URL)
	if err == nil {
		t.Fatalf("ResolveStreamURL returned nil error, want non-nil for missing File1 entry")
	}
}

func TestResolveStreamURL_NonOKStatusReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(samplePLS))
	}))
	defer srv.Close()

	_, err := ResolveStreamURL(context.Background(), srv.URL)
	if err == nil {
		t.Fatalf("ResolveStreamURL returned nil error, want non-nil for non-OK status")
	}
}

func TestParseBitrateFromURL_ExtractsBitrateAndCodec(t *testing.T) {
	bitrate, codec := ParseBitrateFromURL("https://ice5.somafm.com/dronezone-128-mp3")
	if bitrate != 128 || codec != "MP3" {
		t.Fatalf("ParseBitrateFromURL() = (%d, %q), want (128, \"MP3\")", bitrate, codec)
	}
}

func TestParseBitrateFromURL_NoMatchReturnsZeroValue(t *testing.T) {
	bitrate, codec := ParseBitrateFromURL("https://example.test/not-a-stream-url")
	if bitrate != 0 || codec != "" {
		t.Fatalf("ParseBitrateFromURL() = (%d, %q), want (0, \"\")", bitrate, codec)
	}
}
