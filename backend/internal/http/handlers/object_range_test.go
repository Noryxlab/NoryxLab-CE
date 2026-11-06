package handlers

import "testing"

// A range header decides whether a film can be jumped in, so the arithmetic
// is worth pinning down - including the shapes that must be refused rather
// than guessed at.
func TestRequestedRange(t *testing.T) {
	const size = 1000
	for _, item := range []struct {
		name   string
		header string
		start  int64
		end    int64
		ok     bool
	}{
		{"what a player asks first", "bytes=0-", 0, 999, true},
		{"a bounded window", "bytes=200-499", 200, 499, true},
		{"an end past the object stops at the object", "bytes=900-5000", 900, 999, true},
		{"the last bytes, for a trailing index", "bytes=-100", 900, 999, true},
		{"a suffix longer than the object is the whole object", "bytes=-5000", 0, 999, true},
		{"the first range of several", "bytes=0-99,200-299", 0, 99, true},
		{"no header", "", 0, 0, false},
		{"another unit", "items=0-10", 0, 0, false},
		{"a start past the end", "bytes=1000-1200", 0, 0, false},
		{"an end before the start", "bytes=500-200", 0, 0, false},
		{"nonsense", "bytes=abc-def", 0, 0, false},
		{"empty on both sides", "bytes=-", 0, 0, false},
	} {
		t.Run(item.name, func(t *testing.T) {
			start, end, ok := requestedRange(item.header, size)
			if ok != item.ok || (ok && (start != item.start || end != item.end)) {
				t.Fatalf("%q: got (%d, %d, %v), want (%d, %d, %v)",
					item.header, start, end, ok, item.start, item.end, item.ok)
			}
		})
	}
}

// An empty object has no range to serve, and answering one would send a
// Content-Range nobody can satisfy.
func TestAnEmptyObjectHasNoRange(t *testing.T) {
	if _, _, ok := requestedRange("bytes=0-", 0); ok {
		t.Fatal("expected no range on an empty object")
	}
}
