package handlers

import (
	"strconv"
	"strings"
)

// requestedRange reads the first byte range of a Range header, bounded to the
// object's size.
//
// Written rather than taken from the standard library because this route does
// not serve a file or a ReadSeeker: the object lives in a bucket, and the
// range has to reach the object store as a request of its own rather than be
// cut out of bytes already pulled across the network. http.ServeContent would
// do the arithmetic and then read the whole object anyway.
//
// It answers false for anything it does not fully understand - no header, a
// unit other than bytes, a multipart range, a bound that does not parse, a
// start past the end. The caller then serves the whole object with a 200,
// which is what a client that sent a range it cannot have expects least and
// tolerates best.
func requestedRange(header string, size int64) (int64, int64, bool) {
	header = strings.TrimSpace(header)
	if header == "" || size <= 0 {
		return 0, 0, false
	}
	const prefix = "bytes="
	if !strings.HasPrefix(header, prefix) {
		return 0, 0, false
	}
	spec := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	// Only the first range. A multipart answer is legal and no browser asks
	// for one on media; serving the first is what a static file server does.
	if comma := strings.IndexByte(spec, ','); comma >= 0 {
		spec = strings.TrimSpace(spec[:comma])
	}
	dash := strings.IndexByte(spec, '-')
	if dash < 0 {
		return 0, 0, false
	}
	rawStart := strings.TrimSpace(spec[:dash])
	rawEnd := strings.TrimSpace(spec[dash+1:])

	// A suffix range: the last N bytes. Players use it to read a trailing
	// index before deciding what to fetch.
	if rawStart == "" {
		if rawEnd == "" {
			return 0, 0, false
		}
		length, err := strconv.ParseInt(rawEnd, 10, 64)
		if err != nil || length <= 0 {
			return 0, 0, false
		}
		if length > size {
			length = size
		}
		return size - length, size - 1, true
	}

	start, err := strconv.ParseInt(rawStart, 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, false
	}
	end := size - 1
	if rawEnd != "" {
		parsed, err := strconv.ParseInt(rawEnd, 10, 64)
		if err != nil || parsed < start {
			return 0, 0, false
		}
		if parsed < end {
			end = parsed
		}
	}
	return start, end, true
}
