package server

import (
	"io/fs"
	"net/http"
	"os"

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

	// Serve file dữ liệu (video, ảnh, srt...) cho preview.
	dataFS := http.FileServer(http.FS(os.DirFS(s.DataDir)))
	mux.Handle("GET /data/", http.StripPrefix("/data/", dataFS))
}
