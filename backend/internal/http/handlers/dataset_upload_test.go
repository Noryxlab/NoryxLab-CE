package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// An object too large is refused, never quietly trimmed.
//
// The regression this guards is not a crash: the handler used to read the body
// through an io.LimitReader, which stops at the cap instead of failing. An
// oversized upload was stored truncated, answered 201, and was recorded as a
// success. Nothing downstream could tell that object from a complete one.
func TestDatasetUploadRefusesRatherThanTruncates(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/datasets/d/objects/study.zip",
		strings.NewReader("irrelevant"))
	request.ContentLength = maxDatasetObjectBytes + 1

	body, _, ok := datasetUploadBody(recorder, request)
	if ok {
		t.Fatal("an object over the ceiling was accepted; it would be stored truncated")
	}
	if body != nil {
		t.Fatal("a refused upload must not hand back a body to stream")
	}
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("refused with %d, want %d so the caller learns nothing was stored",
			recorder.Code, http.StatusRequestEntityTooLarge)
	}
}

// A body at or below the ceiling is streamed, not buffered.
func TestDatasetUploadStreamsWhatItAccepts(t *testing.T) {
	for _, this := range []struct {
		name     string
		declared int64
		wantSize int64
	}{
		{"a declared length is passed through", 12, 12},
		{"exactly the ceiling still fits", maxDatasetObjectBytes, maxDatasetObjectBytes},
		// A chunked upload declares nothing. Storage is told -1 so it sends in
		// parts; the alternative is reading the object into memory to find out
		// how big it is, which is the bug this replaced.
		{"an undeclared length streams in parts", -1, -1},
	} {
		t.Run(this.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, "/api/v1/datasets/d/objects/f",
				strings.NewReader("hello world!"))
			request.ContentLength = this.declared

			body, size, ok := datasetUploadBody(recorder, request)
			if !ok {
				t.Fatalf("declared length %d was refused", this.declared)
			}
			if size != this.wantSize {
				t.Fatalf("size = %d, want %d", size, this.wantSize)
			}
			if _, isBuffer := body.(interface{ Len() int }); isBuffer {
				t.Fatal("the body was buffered rather than streamed")
			}
		})
	}
}

// A caller who understates Content-Length is still stopped.
//
// The declared length is a claim, so the ceiling has to hold on the stream as
// well as on the header - otherwise announcing one byte and sending ten
// gigabytes walks straight past the check above.
func TestDatasetUploadStopsACallerWhoLiesAboutLength(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/datasets/d/objects/f",
		strings.NewReader(strings.Repeat("x", 4096)))
	request.ContentLength = 8

	body, _, ok := datasetUploadBody(recorder, request)
	if !ok {
		t.Fatal("a small declared length should be accepted at the header")
	}
	defer body.Close()

	// Read past what the reader is allowed to pass through.
	read, err := io.Copy(io.Discard, io.LimitReader(body, 1<<20))
	if err == nil {
		t.Fatalf("read %d bytes with no error; the stream ceiling is not enforced", read)
	}
}

// The timeout follows the size, so a large object over a slow link is not cut
// off midway - which used to leave a part-written object behind.
func TestDatasetUploadDeadlineGrowsWithTheObject(t *testing.T) {
	small := datasetUploadDeadline(1 << 20)
	large := datasetUploadDeadline(2 << 30)
	if large <= small {
		t.Fatalf("a 2 GiB upload gets %s and a 1 MiB one %s; the deadline is flat", large, small)
	}
	if floor := datasetUploadDeadline(-1); floor < 2*time.Minute {
		t.Fatalf("an undeclared length gets %s, below the two-minute floor", floor)
	}
}
