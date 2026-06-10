package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSafePathRejectsTraversalAndEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	s := &server{root: root}

	if full, rel, err := s.safePath(""); err != nil || full != root || rel != "" {
		t.Fatalf("project root rejected: full=%q rel=%q err=%v", full, rel, err)
	}
	if _, _, err := s.safePath("../outside"); err == nil {
		t.Fatal("parent traversal must be rejected")
	}
	if err := os.Symlink(outside, filepath.Join(root, "outside-link")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.safePath("outside-link/secret.txt"); err == nil {
		t.Fatal("symlink escaping the project root must be rejected")
	}
	if _, rel, err := s.safePath("folder/file.txt"); err != nil || rel != "folder/file.txt" {
		t.Fatalf("valid project path rejected: rel=%q err=%v", rel, err)
	}
}

func TestFileOperationRefreshesActivity(t *testing.T) {
	s := &server{root: t.TempDir()}
	s.lastActivity.Store(time.Now().Add(-time.Hour).UnixNano())
	request := httptest.NewRequest(http.MethodGet, "/files", nil)
	response := httptest.NewRecorder()

	s.withActivity(s.get)(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	if idle := s.idleFor(time.Now()); idle > time.Second {
		t.Fatalf("file operation did not refresh activity: %s", idle)
	}
}
