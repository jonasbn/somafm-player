package history

import (
	"testing"
	"time"
)

func TestAdd_MostRecentFirst(t *testing.T) {
	h := New(5)
	h.Add(Entry{Title: "First", PlayedAt: time.Unix(1, 0)})
	h.Add(Entry{Title: "Second", PlayedAt: time.Unix(2, 0)})

	entries := h.Entries()
	if len(entries) != 2 {
		t.Fatalf("Entries() has %d items, want 2", len(entries))
	}
	if entries[0].Title != "Second" || entries[1].Title != "First" {
		t.Fatalf("Entries() = %+v, want [Second, First]", entries)
	}
}

func TestAdd_CapsAtMax(t *testing.T) {
	h := New(5)
	for i := 0; i < 8; i++ {
		h.Add(Entry{Title: string(rune('A' + i)), PlayedAt: time.Unix(int64(i), 0)})
	}
	entries := h.Entries()
	if len(entries) != 5 {
		t.Fatalf("Entries() has %d items, want capped at 5", len(entries))
	}
	if entries[0].Title != "H" {
		t.Fatalf("Entries()[0].Title = %q, want %q (most recent)", entries[0].Title, "H")
	}
}
