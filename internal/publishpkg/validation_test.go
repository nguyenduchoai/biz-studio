package publishpkg

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bizstudio/internal/qc"
	"bizstudio/internal/store"
)

func TestInvalidVideoCannotPublishOrOverwritePreviousPackage(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := st.Settings()
	cfg.ClaudeBin = filepath.Join(dir, "no-provider-cli")
	st.SaveSettings(cfg)
	projectDir := filepath.Join(dir, "projects", "test")
	output := filepath.Join(projectDir, "outputs", "invalid.mp4")
	pub := filepath.Join(projectDir, "publish")
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(pub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("not a video"), 0o600); err != nil {
		t.Fatal(err)
	}
	prior := filepath.Join(pub, "final.mp4")
	if err := os.WriteFile(prior, []byte("previous valid package"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A stale report must never approve a replacement file with the same path.
	if err := os.WriteFile(filepath.Join(projectDir, "qc.json"), []byte(`{"durationS":5,"width":320,"height":240,"warnings":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = Build(ctx, st, &store.Project{ID: "test", Name: "Test", OutputFile: "projects/test/outputs/invalid.mp4"}, projectDir, func(float64, string) {})
	if err == nil {
		t.Error("invalid output passed publish gate")
	}
	got, readErr := os.ReadFile(prior)
	if readErr != nil || string(got) != "previous valid package" {
		t.Errorf("previous package changed despite invalid input: %q, %v", got, readErr)
	}
	if logs := st.Logs(100); len(logs) != 0 {
		t.Fatalf("QC failure reached metadata/provider fallback: %+v", logs)
	}
}

func publishVideoFixture(t *testing.T) (*store.Store, *store.Project, string, string) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := st.Settings()
	cfg.ClaudeBin = filepath.Join(dir, "no-provider-cli")
	st.SaveSettings(cfg)
	p := &store.Project{ID: "test", Name: "Test", OutputFile: "projects/test/outputs/static.mp4"}
	projectDir := filepath.Join(dir, "projects", p.ID)
	src := filepath.Join(dir, p.OutputFile)
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-v", "error", "-y",
		"-f", "lavfi", "-i", "color=c=blue:size=160x120:rate=10:duration=3",
		"-c:v", "libx264", "-preset", "ultrafast", src).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v: %s", err, out)
	}
	return st, p, projectDir, src
}

func TestPublishFreshQCAllowsIntentionalStaticSilentVideo(t *testing.T) {
	st, p, projectDir, src := publishVideoFixture(t)
	pub := filepath.Join(projectDir, "publish")
	if err := os.MkdirAll(pub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pub, "subs.srt"), []byte("stale previous subtitles"), 0o600); err != nil {
		t.Fatal(err)
	}
	zipPath, err := Build(context.Background(), st, p, projectDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(pub, "subs.srt")); !os.IsNotExist(err) {
		t.Fatalf("stale subtitles survived package replacement: %v", err)
	}
	var rep qc.Report
	raw, err := os.ReadFile(filepath.Join(pub, "qc.json"))
	if err != nil || json.Unmarshal(raw, &rep) != nil {
		t.Fatalf("missing current QC report: %s, %v", raw, err)
	}
	hash, err := qc.Fingerprint(context.Background(), src)
	if err != nil || rep.SourceSHA256 != hash || rep.CheckedAt.IsZero() || len(rep.Warnings) == 0 {
		t.Fatalf("unbound report or missing nonblocking warnings: %+v, %v", rep, err)
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	entries := map[string][]byte{}
	for _, file := range zr.File {
		r, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		entries[file.Name], err = io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(entries["final.mp4"]) == 0 || string(entries["qc.json"]) != string(raw) || !strings.Contains(string(entries["meta.json"]), hash) {
		t.Fatalf("zip does not contain its validated video/report/metadata: %v", entries)
	}
}

func TestPublishRejectsChangedBytesAfterQCBeforeMetadata(t *testing.T) {
	st, p, projectDir, src := publishVideoFixture(t)
	pub := filepath.Join(projectDir, "publish")
	if err := os.MkdirAll(pub, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(pub, "final.mp4")
	if err := os.WriteFile(old, []byte("previous package"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Build(context.Background(), st, p, projectDir, func(progress float64, _ string) {
		if progress == 5 {
			if err := os.WriteFile(src, []byte("replaced after fresh QC"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	})
	if err == nil || !strings.Contains(err.Error(), "thay đổi sau QC") {
		t.Fatalf("modified source passed gate: %v", err)
	}
	got, _ := os.ReadFile(old)
	if string(got) != "previous package" {
		t.Fatalf("failure replaced previous package: %q", got)
	}
	for _, entry := range st.Logs(100) {
		if strings.Contains(entry.Message, "Gemini") || strings.Contains(entry.Message, "Claude") {
			t.Fatalf("changed source reached provider call: %+v", entry)
		}
	}
}

func TestPackageInstallFailureRestoresPrevious(t *testing.T) {
	dir := t.TempDir()
	pub := filepath.Join(dir, "publish")
	if err := os.MkdirAll(pub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pub, "final.mp4"), []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installPackage(filepath.Join(dir, "missing-staged"), pub); err == nil {
		t.Fatal("missing stage installed")
	}
	got, err := os.ReadFile(filepath.Join(pub, "final.mp4"))
	if err != nil || string(got) != "previous" {
		t.Fatalf("previous package not restored: %q, %v", got, err)
	}
}

func TestPublishCancellationPreservesPreviousPackage(t *testing.T) {
	st, p, projectDir, _ := publishVideoFixture(t)
	pub := filepath.Join(projectDir, "publish")
	if err := os.MkdirAll(pub, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(pub, "final.mp4")
	if err := os.WriteFile(old, []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := Build(ctx, st, p, projectDir, func(progress float64, _ string) {
		if progress == 85 {
			cancel()
		}
	})
	if err == nil {
		t.Fatal("cancelled package reported success")
	}
	got, err := os.ReadFile(old)
	if err != nil || string(got) != "previous" {
		t.Fatalf("cancelled packaging replaced previous result: %q, %v", got, err)
	}
	stages, _ := filepath.Glob(filepath.Join(projectDir, ".publish-*"))
	if len(stages) != 0 {
		t.Fatalf("abandoned staging files: %v", stages)
	}
}

func TestPublishSourceRejectsEscapedAndSymlinkedOutput(t *testing.T) {
	base, external := t.TempDir(), t.TempDir()
	for _, rel := range []string{"../outside.mp4", filepath.Join(external, "outside.mp4"), "https://example.invalid/video.mp4"} {
		if _, err := publishSource(base, rel); err == nil {
			t.Errorf("accepted escaped source %q", rel)
		}
	}
	outside := filepath.Join(external, "outside.mp4")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(base, "link.mp4")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := publishSource(base, "link.mp4"); err == nil {
		t.Fatal("symlink outside data directory accepted")
	}
}
