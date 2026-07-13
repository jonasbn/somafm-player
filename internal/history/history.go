package history

import "time"

type Entry struct {
	Title    string
	Artist   string
	Channel  string
	PlayedAt time.Time
}

type History struct {
	entries []Entry
	max     int
}

func New(max int) *History {
	return &History{max: max}
}

func (h *History) Add(e Entry) {
	h.entries = append([]Entry{e}, h.entries...)
	if len(h.entries) > h.max {
		h.entries = h.entries[:h.max]
	}
}

func (h *History) Entries() []Entry {
	out := make([]Entry, len(h.entries))
	copy(out, h.entries)
	return out
}
