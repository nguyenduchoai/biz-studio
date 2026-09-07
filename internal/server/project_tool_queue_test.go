package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bizstudio/internal/store"
	"bizstudio/internal/vtemplate"
)

func TestProjectBackedToolsWaitForLeaseWithoutBlockingOtherProject(t *testing.T) {
	for _, kind := range []string{"asr", "normalize", "dub"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			st, err := store.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			cfg := st.Settings()
			cfg.Threads = 2
			st.SaveSettings(cfg)
			s := New(st, dir, 6868, 6869)
			for _, id := range []string{"project-a", "project-b"} {
				p := store.Project{ID: id, Name: id}
				st.SaveProject(&p)
			}
			path := filepath.Join(s.ProjectDir("project-a"), "outputs", "video.mp4")
			// If accidentally invoked, FFmpeg fails before any provider call.
			if err := os.WriteFile(path, []byte("intentionally invalid no-provider fixture"), 0o600); err != nil {
				t.Fatal(err)
			}
			release, ok := st.ProjectWork.TryAcquire("project-a")
			if !ok {
				t.Fatal("could not hold project lease")
			}
			t.Cleanup(func() {
				release()
				deadline := time.Now().Add(10 * time.Second)
				for time.Now().Before(deadline) {
					busy := false
					for _, job := range st.Jobs() {
						busy = busy || job.Status == "running" || job.Status == "queued"
					}
					if !busy {
						return
					}
					time.Sleep(10 * time.Millisecond)
				}
				t.Error("fixture jobs did not settle")
			})
			body := map[string]string{"path": s.toolRelPath(path), "engine": "gemini", "platform": vtemplate.Platforms()[0].ID}
			if kind == "dub" {
				// A global video plus project-owned SRT must still protect the
				// SRT directory (translation writes neighboring subtitles).
				globalVideo := filepath.Join(dir, "global.mp4")
				for _, file := range []string{globalVideo, path + ".srt"} {
					if err := os.WriteFile(file, []byte("invalid no-provider fixture"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				body["videoPath"], body["srtPath"] = globalVideo, path+".srt"
			}
			raw, _ := json.Marshal(body)
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(raw)))
			w := httptest.NewRecorder()
			switch kind {
			case "asr":
				s.handleToolASR(w, r)
			case "normalize":
				s.handleNormalize(w, r)
			case "dub":
				s.handleToolDub(w, r)
			}
			if w.Code != http.StatusOK {
				t.Fatalf("submit: %d %s", w.Code, w.Body.String())
			}
			var submitted store.Job
			if err := json.Unmarshal(w.Body.Bytes(), &submitted); err != nil {
				t.Fatal(err)
			}
			if submitted.ProjectID != "project-a" {
				t.Errorf("project-owned %s job escaped serialization: %+v", kind, submitted)
			}
			otherStarted := make(chan struct{})
			s.Jobs.Submit("other", "project-b", "", func(func(float64, string)) (string, error) {
				close(otherStarted)
				return "", nil
			})
			select {
			case <-otherStarted:
			case <-time.After(10 * time.Second):
				t.Fatal("busy project blocked unrelated project")
			}
			current, _ := st.Job(submitted.ID)
			if current.Status != "queued" || current.Progress != 0 {
				t.Errorf("tool invoked while project lease held: %+v", current)
			}
		})
	}
}

func TestProjectToolPathUsesCanonicalContainedExistingProject(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := New(st, dir, 6868, 6869)
	project := store.Project{ID: "project-a", Name: "A"}
	st.SaveProject(&project)
	outputs := filepath.Join(s.ProjectDir(project.ID), "outputs")
	for _, path := range []string{outputs, "projects/project-a/outputs", filepath.Join(outputs, "..", "outputs")} {
		if got := s.projectIDForToolPath(path); got != project.ID {
			t.Errorf("project path %q mapped to %q", path, got)
		}
	}
	for _, rel := range []string{"uploads", "projects/missing/outputs", "projects-archive/project-a/outputs"} {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if got := s.projectIDForToolPath(path); got != "" {
			t.Errorf("global/unknown path %q acquired project %q", path, got)
		}
	}
	caseAlias := filepath.Join(dir, "projects", "PROJECT-A", "outputs")
	if _, err := os.Stat(caseAlias); err == nil {
		if got := s.projectIDForToolPath(caseAlias); got != project.ID {
			t.Errorf("case-insensitive alias bypassed project lease: %q", got)
		}
	}
	external := t.TempDir()
	insideAlias := filepath.Join(external, "inside-alias")
	if err := os.Symlink(outputs, insideAlias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if got := s.projectIDForToolPath(insideAlias); got != project.ID {
		t.Fatalf("alias to project directory bypassed its lease: %q", got)
	}
	outsideAlias := filepath.Join(outputs, "outside-alias")
	if err := os.Symlink(external, outsideAlias); err != nil {
		t.Fatal(err)
	}
	if got := s.projectIDForToolPath(outsideAlias); got != "" {
		t.Fatalf("escaped directory mapped to project: %q", got)
	}
}
