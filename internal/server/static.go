package server

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"bizstudio/web"
)

func (s *Server) routesStatic(mux *http.ServeMux) {
	staticFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(staticFS))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			b, _ := fs.ReadFile(staticFS, "index.html")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	// Chỉ serve artifact/media mà giao diện cần preview. Không được dùng
	// FileServer trên toàn DataDir: db.json chứa API keys, instance marker và
	// venv/setup scripts không phải tài nguyên công khai.
	mux.HandleFunc("GET /data/{path...}", s.handlePublicDataFile)
}

var publicDataRoots = map[string]bool{
	"projects": true, "downloads": true, "uploads": true, "styles": true,
	"characters": true, "text2video": true, "tmp": true, "avatar": true,
	"veo": true, "music": true, "sfx": true, "recap": true,
}

var publicDataExtensions = map[string]bool{
	".mp4": true, ".mov": true, ".mkv": true, ".webm": true, ".avi": true,
	".m4v": true, ".flv": true, ".wmv": true, ".mpg": true, ".mpeg": true,
	".mp3": true, ".wav": true, ".m4a": true, ".aac": true, ".flac": true,
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
	".srt": true, ".vtt": true, ".ass": true, ".sub": true,
	".txt": true, ".json": true, ".zip": true,
}

func (s *Server) handlePublicDataFile(w http.ResponseWriter, r *http.Request) {
	rel := filepath.Clean(filepath.FromSlash(r.PathValue("path")))
	if rel == "." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 2 || !publicDataRoots[parts[0]] {
		http.NotFound(w, r)
		return
	}
	for _, part := range parts {
		if part == "" || strings.HasPrefix(part, ".") {
			http.NotFound(w, r)
			return
		}
	}
	if !publicDataExtensions[strings.ToLower(filepath.Ext(rel))] {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(s.DataDir, rel)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	http.ServeFile(w, r, path)
}
