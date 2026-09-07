package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"bizstudio/internal/media"
	"bizstudio/internal/store"
	"bizstudio/internal/timeline"
)

func renderRouteFixture(t *testing.T) (*Server, store.Project, string, string) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
	dataDir := t.TempDir()
	st, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	s := New(st, dataDir, 6868, 6869)
	p := store.Project{ID: "render-validation", Name: "Validation", Width: 160, Height: 120, Status: "ready", Progress: 2}
	dir := s.ProjectDir(p.ID)
	src := filepath.Join(dir, "assets", "source.mp4")
	p.OutputFile = s.toolRelPath(src)
	st.SaveProject(&p)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-v", "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=size=160x120:rate=10:duration=2",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-movflags", "+faststart", "-shortest", src).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v: %s", err, out)
	}
	doc := timeline.Doc{ProjectID: p.ID, Video: p.OutputFile, VideoDur: 2,
		Tracks: []timeline.Track{{ID: "src", Role: timeline.RoleSource, Name: "Source"}}}
	doc.Normalize()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.timelinePath(p.ID), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return s, p, dir, src
}

func waitRenderRouteJob(t *testing.T, s *Server, handler http.HandlerFunc, projectID string) store.Job {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.SetPathValue("id", projectID)
	w := httptest.NewRecorder()
	handler(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("submit: %d %s", w.Code, w.Body.String())
	}
	var submitted store.Job
	if err := json.Unmarshal(w.Body.Bytes(), &submitted); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		job, _ := s.st.Job(submitted.ID)
		if job.Status == "done" || job.Status == "error" {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("render job did not complete within a minute")
	return store.Job{}
}

func TestVideoRenderRoutesCommitValidatedOutput(t *testing.T) {
	for _, kind := range []string{"final", "timeline"} {
		t.Run(kind, func(t *testing.T) {
			s, p, dir, _ := renderRouteFixture(t)
			handler := s.handleProjectRenderFinal
			if kind == "timeline" {
				handler = s.handleTimelineRender
			}
			// Repeat against an existing destination: especially important on
			// Windows, where an open/live output must not be truncated in place.
			for i := 0; i < 2; i++ {
				job := waitRenderRouteJob(t, s, handler, p.ID)
				if job.Status != "done" {
					t.Fatalf("render %d: %+v", i, job)
				}
				current, _ := s.st.Project(p.ID)
				want := "projects/" + p.ID + "/outputs/" + kind + ".mp4"
				if current.OutputFile != want || job.Output != want || current.Status != "done" || current.Progress != 6 {
					t.Fatalf("validated output not committed: project=%+v job=%+v", current, job)
				}
				if err := media.ValidateVideo(context.Background(), filepath.Join(s.DataDir, want)); err != nil {
					t.Fatalf("done project is not a playable video: %v", err)
				}
			}
			leftovers, err := filepath.Glob(filepath.Join(dir, "tmp", "*-render-*.mp4"))
			if err != nil || len(leftovers) != 0 {
				t.Fatalf("temporary render files not cleaned: %v, %v", leftovers, err)
			}
		})
	}
}

func TestValidateAndPromoteRenderPreservesPreviousOnInvalidOrCancelled(t *testing.T) {
	_, _, dir, src := renderRouteFixture(t)
	previous, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "outputs", "final.mp4")
	if err := os.WriteFile(dst, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]byte{nil, []byte("not video"), previous[:len(previous)*2/3]} {
		tmp, err := newRenderTemp(dir, "invalid-*.mp4")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(tmp, invalid, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := validateAndPromoteRender(context.Background(), tmp, dst); err == nil {
			t.Fatal("invalid output promoted")
		}
		got, _ := os.ReadFile(dst)
		if string(got) != string(previous) {
			t.Fatal("previous output overwritten by invalid render")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := validateAndPromoteRender(ctx, src, dst); err == nil {
		t.Fatal("cancelled render promoted")
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("cancelled render moved source: %v", err)
	}
}

func TestTimelineRenderFailurePreservesPriorProjectOutput(t *testing.T) {
	s, p, dir, src := renderRouteFixture(t)
	job := waitRenderRouteJob(t, s, s.handleTimelineRender, p.ID)
	if job.Status != "done" {
		t.Fatalf("first render: %+v", job)
	}
	before, _ := s.st.Project(p.ID)
	dst := filepath.Join(dir, "outputs", "timeline.mp4")
	prior, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("broken source"), 0o600); err != nil {
		t.Fatal(err)
	}
	job = waitRenderRouteJob(t, s, s.handleTimelineRender, p.ID)
	if job.Status != "error" {
		t.Fatalf("invalid timeline source marked done: %+v", job)
	}
	after, _ := s.st.Project(p.ID)
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != string(prior) || after.OutputFile != before.OutputFile || after.UpdatedAt != before.UpdatedAt {
		t.Fatalf("failed render changed previous file/project: %v, before=%+v after=%+v", err, before, after)
	}
}
