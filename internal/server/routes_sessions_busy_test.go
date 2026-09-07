package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"bizstudio/internal/agent"
	"bizstudio/internal/store"
)

func TestSessionAdmissionConflictReturns409WithoutLaunchingAI(t *testing.T) {
	s := newTestServer(t)
	// The application runner is process-wide; isolate it without starting any
	// real subprocess, then restore lazy initialization for subsequent tests.
	previous := aiRunner
	aiRunner = agent.New(s.st, s.Hub.Broadcast, s.DataDir)
	aiOnce = sync.Once{}
	aiOnce.Do(func() {})
	t.Cleanup(func() {
		aiRunner = previous
		aiOnce = sync.Once{}
		if previous != nil {
			aiOnce.Do(func() {})
		}
	})
	p := store.Project{Name: "Busy"}
	s.st.SaveProject(&p)
	sess := store.Session{ProjectID: p.ID, Status: "stopped", ClaudeSessionID: "resume-test"}
	s.st.SaveSession(&sess)
	release, ok := s.st.ProjectWork.TryAcquire(p.ID)
	if !ok {
		t.Fatal("could not acquire project lease")
	}
	defer release()
	for _, endpoint := range []struct{ path, body string }{
		{"/api/projects/" + p.ID + "/sessions", ""},
		{"/api/sessions/" + sess.ID + "/message", `{"text":"Tiếp tục"}`},
	} {
		r := httptest.NewRequest(http.MethodPost, endpoint.path, strings.NewReader(endpoint.body))
		markLocalControlRequest(r)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != http.StatusConflict {
			t.Errorf("%s: got %d, want 409: %s", endpoint.path, w.Code, w.Body.String())
		}
	}
	if len(s.st.SessionsByProject(p.ID)) != 1 {
		t.Fatal("busy admission created a phantom session")
	}
}
