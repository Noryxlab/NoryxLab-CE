package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type fileItem struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	IsDirectory bool      `json:"isDirectory"`
	Size        int64     `json:"size"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type server struct {
	root string
}

func main() {
	root := flag.String("root", "/mnt", "project file root")
	listen := flag.String("listen", ":8080", "listen address")
	flag.Parse()

	absolute, err := filepath.Abs(*root)
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(absolute, 0o770); err != nil {
		panic(err)
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		panic(err)
	}
	s := server{root: absolute}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /files", s.get)
	mux.HandleFunc("POST /folders", s.createFolder)
	mux.HandleFunc("GET /files/{path...}", s.get)
	mux.HandleFunc("PUT /files/{path...}", s.put)
	mux.HandleFunc("DELETE /files/{path...}", s.delete)
	if err := http.ListenAndServe(*listen, mux); err != nil {
		panic(err)
	}
}

func (s server) safePath(raw string) (string, string, error) {
	rel := strings.Trim(strings.TrimSpace(raw), "/")
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." {
		clean = ""
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", errors.New("path escapes project root")
	}
	full := filepath.Join(s.root, clean)
	if clean == "" {
		return s.root, "", nil
	}
	resolvedParent, err := filepath.EvalSymlinks(filepath.Dir(full))
	if err == nil && resolvedParent != s.root && !strings.HasPrefix(resolvedParent, s.root+string(filepath.Separator)) {
		return "", "", errors.New("path escapes project root")
	}
	if resolved, err := filepath.EvalSymlinks(full); err == nil && resolved != s.root && !strings.HasPrefix(resolved, s.root+string(filepath.Separator)) {
		return "", "", errors.New("path escapes project root")
	}
	return full, filepath.ToSlash(clean), nil
}

func (s server) get(w http.ResponseWriter, r *http.Request) {
	full, rel, err := s.safePath(r.PathValue("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	info, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !info.IsDir() {
		contentType := mime.TypeByExtension(filepath.Ext(info.Name()))
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, info.Name()))
		http.ServeFile(w, r, full)
		return
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	items := make([]fileItem, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		path := filepath.ToSlash(filepath.Join(rel, entry.Name()))
		items = append(items, fileItem{
			Name:        entry.Name(),
			Path:        path,
			IsDirectory: entry.IsDir(),
			Size:        info.Size(),
			UpdatedAt:   info.ModTime().UTC(),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDirectory != items[j].IsDirectory {
			return items[i].IsDirectory
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	writeJSON(w, http.StatusOK, map[string]any{"path": rel, "items": items})
}

func (s server) put(w http.ResponseWriter, r *http.Request) {
	full, rel, err := s.safePath(r.PathValue("path"))
	if err != nil || rel == "" {
		writeError(w, http.StatusBadRequest, errors.New("valid file path is required"))
		return
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o770); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(full), ".noryx-upload-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, r.Body); err != nil {
		_ = tmp.Close()
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := tmp.Chmod(0o660); err != nil {
		_ = tmp.Close()
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := tmp.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := os.Rename(tmpName, full); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"path": rel})
}

func (s server) createFolder(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Path string `json:"path"`
	}
	if json.NewDecoder(r.Body).Decode(&request) != nil {
		writeError(w, http.StatusBadRequest, errors.New("valid path is required"))
		return
	}
	full, rel, err := s.safePath(request.Path)
	if err != nil || rel == "" {
		writeError(w, http.StatusBadRequest, errors.New("valid folder path is required"))
		return
	}
	if err := os.MkdirAll(full, 0o770); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"path": rel})
}

func (s server) delete(w http.ResponseWriter, r *http.Request) {
	full, rel, err := s.safePath(r.PathValue("path"))
	if err != nil || rel == "" {
		writeError(w, http.StatusBadRequest, errors.New("valid path is required"))
		return
	}
	if err := os.RemoveAll(full); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
