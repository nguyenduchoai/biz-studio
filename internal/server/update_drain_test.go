package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bizstudio/internal/store"
)

func TestUpdateApplyDrainsRequestBeforeCheckingNewJob(t *testing.T) {
	s := newTestServer(t)
	s.Updater = nil
	entered, release := make(chan struct{}), make(chan struct{})
	s.mux.HandleFunc("POST /api/test-create-job", func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		s.st.SaveJob(&store.Job{Status: "running"})
		w.WriteHeader(http.StatusAccepted)
	})
	request := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, path, nil)
		markLocalControlRequest(r)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	jobDone := make(chan struct{})
	go func() { request("/api/test-create-job"); close(jobDone) }()
	<-entered
	applyDone := make(chan *httptest.ResponseRecorder, 1)
	go func() { applyDone <- request("/api/update/apply") }()
	select {
	case <-applyDone:
		t.Fatal("update ran before the request that publishes a job completed")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	<-jobDone
	response := <-applyDone
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "tác vụ") {
		t.Fatalf("newly published job was missed: %d %s", response.Code, response.Body.String())
	}
}

func TestUpdateReservationRejectsControlAndMobileWrites(t *testing.T) {
	s := newTestServer(t)
	_, cancel, ok := beginSetup("app-update", time.Minute)
	if !ok {
		t.Fatal("setup reservation unavailable")
	}
	defer endSetup("app-update", cancel)
	for _, tc := range []struct {
		path string
		http.Handler
	}{{"/api/projects", s}, {"/m/project/upload", s.MobileHandler()}} {
		r := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"name":"must not save"}`))
		markLocalControlRequest(r)
		w := httptest.NewRecorder()
		tc.ServeHTTP(w, r)
		if w.Code != http.StatusConflict {
			t.Fatalf("%s: status=%d body=%s", tc.path, w.Code, w.Body.String())
		}
	}
	if len(s.st.Projects()) != 0 {
		t.Fatal("mutation was allowed during update")
	}
}
