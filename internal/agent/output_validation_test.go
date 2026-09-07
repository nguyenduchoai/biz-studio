package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeAgentOutput(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeAgentMeta(t *testing.T, root, status, output string) {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{"status": status, "output": output})
	writeAgentOutput(t, root, "meta.json", string(raw))
}

func TestOutputGateRejectsMissingStaleAndUnsafeArtifacts(t *testing.T) {
	for _, name := range []string{"missing-meta", "broken-meta", "wrong-status", "missing-video", "directory", "empty", "traversal", "nested-traversal", "absolute", "windows-drive", "symlink", "symlink-dir", "meta-symlink", "old", "renamed-old", "touched-old", "invalid-video"} {
		t.Run(name, func(t *testing.T) {
			r, p, _ := testRunner(t)
			root := filepath.Join(r.dataDir, "projects", p.ID)
			writeAgentOutput(t, root, "outputs/old.mp4", "old video")
			before, err := r.snapshotOutputs(context.Background(), p.ID)
			if err != nil {
				t.Fatal(err)
			}
			r.validateVideo = func(context.Context, string) error { return nil }
			writeAgentOutput(t, root, "outputs/new.mp4", "new video")
			writeAgentMeta(t, root, "done", "outputs/new.mp4")
			switch name {
			case "missing-meta":
				_ = os.Remove(filepath.Join(root, "meta.json"))
			case "broken-meta":
				writeAgentOutput(t, root, "meta.json", "not JSON")
			case "wrong-status":
				writeAgentMeta(t, root, "running", "outputs/new.mp4")
			case "missing-video":
				writeAgentMeta(t, root, "done", "outputs/missing.mp4")
			case "directory":
				_ = os.Mkdir(filepath.Join(root, "outputs", "dir.mp4"), 0o700)
				writeAgentMeta(t, root, "done", "outputs/dir.mp4")
			case "empty":
				writeAgentOutput(t, root, "outputs/new.mp4", "")
			case "traversal":
				writeAgentMeta(t, root, "done", "../outside.mp4")
			case "nested-traversal":
				writeAgentMeta(t, root, "done", "outputs/../outputs/new.mp4")
			case "absolute":
				writeAgentMeta(t, root, "done", filepath.Join(root, "outputs", "new.mp4"))
			case "windows-drive":
				writeAgentMeta(t, root, "done", `C:\outputs\new.mp4`)
			case "symlink", "symlink-dir", "meta-symlink":
				outside := t.TempDir()
				writeAgentOutput(t, outside, "other.mp4", "different video")
				target, link := filepath.Join(outside, "other.mp4"), filepath.Join(root, "outputs", "link.mp4")
				output := "outputs/link.mp4"
				if name == "symlink-dir" {
					target, link, output = outside, filepath.Join(root, "outputs", "linked"), "outputs/linked/other.mp4"
				}
				if name == "meta-symlink" {
					target, link = filepath.Join(outside, "meta.json"), filepath.Join(root, "meta.json")
					writeAgentMeta(t, outside, "done", "outputs/new.mp4")
					_ = os.Remove(link)
				}
				if err := os.Symlink(target, link); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				if name != "meta-symlink" {
					writeAgentMeta(t, root, "done", output)
				}
			case "old":
				writeAgentMeta(t, root, "done", "outputs/old.mp4")
			case "renamed-old":
				writeAgentOutput(t, root, "outputs/new.mp4", "old video")
			case "touched-old":
				_ = os.Chtimes(filepath.Join(root, "outputs", "old.mp4"), time.Now(), time.Now())
				writeAgentMeta(t, root, "done", "outputs/old.mp4")
			case "invalid-video":
				r.validateVideo = func(context.Context, string) error { return errors.New("video decode failed") }
			}
			if out, err := r.verifiedOutput(context.Background(), p.ID, before); err == nil {
				t.Fatalf("accepted %s: %q", name, out)
			}
		})
	}
}

func TestOutputGateAcceptsOnlyFreshVerifiedContent(t *testing.T) {
	r, p, _ := testRunner(t)
	root := filepath.Join(r.dataDir, "projects", p.ID)
	writeAgentOutput(t, root, "outputs/video.mp4", "original")
	before, err := r.snapshotOutputs(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	writeAgentOutput(t, root, "outputs/video.mp4", "revised video")
	writeAgentMeta(t, root, "done", `outputs\video.mp4`)
	called := false
	r.validateVideo = func(ctx context.Context, name string) error { called = true; return nil }
	out, err := r.verifiedOutput(context.Background(), p.ID, before)
	if err != nil || !called || out != "projects/"+p.ID+"/outputs/video.mp4" {
		t.Fatalf("out=%q err=%v validated=%v", out, err, called)
	}
}

func TestOutputGateRejectsWritesDuringValidation(t *testing.T) {
	r, p, _ := testRunner(t)
	root := filepath.Join(r.dataDir, "projects", p.ID)
	writeAgentOutput(t, root, "outputs/video.mp4", "new video")
	writeAgentMeta(t, root, "done", "outputs/video.mp4")
	r.validateVideo = func(context.Context, string) error {
		writeAgentOutput(t, root, "outputs/video.mp4", "still rendering")
		return nil
	}
	if _, err := r.verifiedOutput(context.Background(), p.ID, nil); err == nil || !strings.Contains(err.Error(), "thay đổi") {
		t.Fatalf("accepted changing artifact: %v", err)
	}
}

func TestOutputFingerprintHonorsCancellation(t *testing.T) {
	r, p, _ := testRunner(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.snapshotOutputs(ctx, p.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("snapshot error=%v", err)
	}
}
