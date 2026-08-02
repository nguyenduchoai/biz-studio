package server

import (
	"net/http"
	"os"
	"path/filepath"

	"bizstudio/internal/jobs"
	"bizstudio/internal/store"
)

// Server — HTTP server chính của Biz Studio.
type Server struct {
	st      *store.Store
	mux     *http.ServeMux
	Hub     *Hub
	Jobs    *jobs.Manager
	DataDir string
	Port    int
}

func New(st *store.Store, dataDir string, port int) *Server {
	abs, _ := filepath.Abs(dataDir)
	s := &Server{
		st:      st,
		mux:     http.NewServeMux(),
		Hub:     NewHub(),
		DataDir: abs,
		Port:    port,
	}
	s.Jobs = jobs.New(st, s.Hub.Broadcast)

	s.routesState(s.mux)
	s.routesStatic(s.mux)
	s.routesSettings(s.mux)
	s.routesProjects(s.mux)
	s.routesAssets(s.mux)
	s.routesSessions(s.mux)
	s.routesTools(s.mux)
	s.routesMisc(s.mux)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ProjectDir trả (và tạo) thư mục dự án data/projects/<id>.
func (s *Server) ProjectDir(id string) string {
	dir := filepath.Join(s.DataDir, "projects", id)
	for _, sub := range []string{"assets", "outputs", "publish", "tmp"} {
		_ = os.MkdirAll(filepath.Join(dir, sub), 0o755)
	}
	return dir
}

// Log ghi nhật ký + phát SSE.
func (s *Server) Log(level, module, msg string) {
	e := s.st.AddLog(level, module, msg)
	s.Hub.Broadcast("log", e)
}
