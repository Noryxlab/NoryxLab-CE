package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafePathRejectsTraversalAndEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	s := server{root: root}

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
