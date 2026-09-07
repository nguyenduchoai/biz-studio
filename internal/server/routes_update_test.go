package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"bizstudio/internal/store"
)

func TestUpdateApplyRejectsBusyWorkBeforeStartingUpdater(t *testing.T) {
	for _, kind := range []string{"job-running", "job-queued", "session", "session-stopping", "idea", "setup"} {
		t.Run(kind, func(t *testing.T) {
			s := newTestServer(t)
			// A blocked request must not even request a stage or launch a helper.
			s.Updater = nil
			switch kind {
			case "job-running", "job-queued":
				s.st.SaveJob(&store.Job{Status: strings.TrimPrefix(kind, "job-")})
			case "session":
				p := store.Project{Name: "Active project"}
				s.st.SaveProject(&p)
				s.st.SaveSession(&store.Session{ProjectID: p.ID, Status: "running"})
			case "session-stopping":
				p := store.Project{Name: "Stopping project"}
				s.st.SaveProject(&p)
				s.st.SaveSession(&store.Session{ProjectID: p.ID, Status: "stopped"})
				release, ok := s.st.ProjectWork.TryAcquire(p.ID)
				if !ok {
					t.Fatal("cannot acquire project lease")
				}
				defer release()
			case "idea":
				s.st.SaveIdea(&store.Idea{Status: "producing"})
			case "setup":
				_, cancel, ok := beginSetup("update-test", time.Minute)
				if !ok {
					t.Fatal("setup slot unavailable")
				}
				defer endSetup("update-test", cancel)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/update/apply", nil)
			markLocalControlRequest(req)
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, req)
			if rec.Code != http.StatusConflict {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestUpdateApplyRejectsUnsavedDataBeforeStartingUpdater(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := New(st, dir, 6868, 6869)
	s.Updater = nil
	offline := s.DataDir + "-offline"
	if err := os.Rename(s.DataDir, offline); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(offline, s.DataDir) })
	s.st.SaveProject(&store.Project{Name: "Cannot persist"})
	if s.st.PersistenceError() == "" {
		t.Fatal("failed to trigger persistence error")
	}
	req := httptest.NewRequest(http.MethodPost, "/api/update/apply", nil)
	markLocalControlRequest(req)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUpdateLaunchArgsPreservesWindowModeWithoutReusingOtherFlags(t *testing.T) {
	for _, flags := range [][]string{{"-window=false"}, {"--window=false"}, {"-test.run=Test", "-data", "old", "-window=false"}} {
		got := updateLaunchArgs(6970, "custom data", flags)
		want := []string{"-port", "6970", "-data", "custom data", "-window=false"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("flags=%v got=%v want=%v", flags, got, want)
		}
	}
	got := updateLaunchArgs(6970, "-window=false", []string{"-data", "-window=false"})
	if len(got) != 4 {
		t.Fatalf("data directory value was parsed as a window flag: %v", got)
	}
}
