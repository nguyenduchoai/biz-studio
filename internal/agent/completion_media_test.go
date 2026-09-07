package agent

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCompletionGateUsesRealVideoDecode(t *testing.T) {
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s unavailable", bin)
		}
	}
	for _, kind := range []string{"valid", "audio-only", "corrupt"} {
		t.Run(kind, func(t *testing.T) {
			r, p, s := testRunner(t)
			root := filepath.Join(r.dataDir, "projects", p.ID)
			before, err := r.snapshotOutputs(context.Background(), p.ID)
			if err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(root, "outputs", "video.mp4")
			if kind == "corrupt" {
				writeAgentOutput(t, root, "outputs/video.mp4", "not a video")
			} else {
				args := []string{"-nostdin", "-v", "error", "-y", "-f", "lavfi"}
				if kind == "audio-only" {
					args = append(args, "-i", "sine=frequency=440:duration=0.4", "-c:a", "aac")
				} else {
					args = append(args, "-i", "testsrc2=size=160x120:rate=10:duration=0.4", "-c:v", "libx264", "-preset", "ultrafast")
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				if out, err := exec.CommandContext(ctx, "ffmpeg", append(args, file)...).CombinedOutput(); err != nil {
					t.Fatalf("fixture failed: %v %s", err, out)
				}
			}
			if _, err := os.Stat(file); err != nil {
				t.Fatal(err)
			}
			writeAgentMeta(t, root, "done", "outputs/video.mp4")
			r.finishRun(context.Background(), s.ID, p.ID, &streamEvent{Subtype: "success"}, nil, &bytes.Buffer{}, before)
			s, _ = r.st.Session(s.ID)
			p, _ = r.st.Project(p.ID)
			want := "error"
			if kind == "valid" {
				want = "done"
			}
			if s.Status != want || p.Status != want {
				t.Fatalf("%s accepted incorrectly: %s/%s", kind, s.Status, p.Status)
			}
		})
	}
}
