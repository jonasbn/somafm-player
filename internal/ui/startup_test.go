package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const sampleChannelsJSONForRetryTest = `{
  "channels": [
    {"id": "dronezone", "title": "Drone Zone", "genre": "ambient"}
  ]
}`

func TestFetchChannelsCmd_ParsesFromHTTPServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(sampleChannelsJSONForRetryTest))
	}))
	defer srv.Close()

	msg := fetchChannelsCmd(srv.URL)()
	fetched, ok := msg.(channelsFetchedMsg)
	if !ok {
		t.Fatalf("expected channelsFetchedMsg, got %T", msg)
	}
	if fetched.err != nil {
		t.Fatalf("unexpected error: %v", fetched.err)
	}
	if len(fetched.channels) != 1 || fetched.channels[0].Title != "Drone Zone" {
		t.Fatalf("fetched.channels = %+v, want a single Drone Zone channel", fetched.channels)
	}
}

func TestFetchChannelsCmd_ServerErrorReportsErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	msg := fetchChannelsCmd(srv.URL)()
	fetched, ok := msg.(channelsFetchedMsg)
	if !ok {
		t.Fatalf("expected channelsFetchedMsg, got %T", msg)
	}
	if fetched.err == nil {
		t.Fatal("expected non-nil err for a 500 response, got nil")
	}
}
