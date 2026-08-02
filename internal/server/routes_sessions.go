package server

import (
	"net/http"
	"sync"

	"bizstudio/internal/agent"
	"bizstudio/internal/store"
)

// aiRunner — singleton agent.Runner cho toàn server (khởi tạo lười).
var (
	aiRunner *agent.Runner
	aiOnce   sync.Once
)

// runner trả về agent.Runner singleton, khởi tạo lần đầu từ Server.
func (s *Server) runner() *agent.Runner {
	aiOnce.Do(func() {
		aiRunner = agent.New(s.st, s.Hub.Broadcast, s.DataDir)
	})
	return aiRunner
}

func (s *Server) routesSessions(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/projects/{id}/sessions", s.handleSessionStart)
	mux.HandleFunc("POST /api/sessions/{id}/message", s.handleSessionMessage)
	mux.HandleFunc("POST /api/sessions/{id}/stop", s.handleSessionStop)
	mux.HandleFunc("GET /api/sessions/{id}/events", s.handleSessionEvents)
}

func (s *Server) handleSessionStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.st.Project(id); !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy dự án %q", id)
		return
	}
	var body struct {
		Extra string `json:"extra"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(r, &body); err != nil {
			httpErr(w, http.StatusBadRequest, "%s", err)
			return
		}
	}
	sess, err := s.runner().Start(id, body.Extra)
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "không khởi động được phiên AI: %s", err)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleSessionMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Text string `json:"text"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	if err := s.runner().Resume(id, body.Text); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (s *Server) handleSessionStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.runner().Stop(id); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSessionEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.st.Session(id); !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy phiên %q", id)
		return
	}
	events := s.st.EventsBySession(id)
	if events == nil {
		events = []store.SessionEvent{}
	}
	writeJSON(w, http.StatusOK, events)
}
