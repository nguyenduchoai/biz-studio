package media

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func requireValidationTools(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
}

func validationFixture(t *testing.T, path string, audioOnly bool) {
	t.Helper()
	args := []string{"-nostdin", "-v", "error", "-y", "-f", "lavfi"}
	if audioOnly {
		args = append(args, "-i", "sine=frequency=440:duration=1", "-c:a", "aac")
	} else {
		args = append(args, "-i", "testsrc2=size=160x120:rate=10:duration=2", "-c:v", "libx264", "-preset", "ultrafast")
	}
	args = append(args, "-movflags", "+faststart", path)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v: %s", err, out)
	}
}

func TestValidateVideoDecodesActualLocalVideo(t *testing.T) {
	requireValidationTools(t)
	dir := t.TempDir()
	valid := filepath.Join(dir, "Tiếng Việt space 'comma,[x].mp4")
	validationFixture(t, valid, false)
	if err := ValidateVideo(context.Background(), valid); err != nil {
		t.Fatalf("valid video without audio rejected: %v", err)
	}
	audio := filepath.Join(dir, "audio.m4a")
	validationFixture(t, audio, true)
	if err := ValidateVideo(context.Background(), audio); err == nil {
		t.Fatal("audio-only file accepted as video")
	}
	bytes, err := os.ReadFile(valid)
	if err != nil {
		t.Fatal(err)
	}
	truncated := filepath.Join(dir, "truncated.mp4")
	if err := os.WriteFile(truncated, bytes[:len(bytes)*2/3], 0o600); err != nil {
		t.Fatal(err)
	}
	// Faststart keeps the original duration/dimensions at the front. A probe
	// still succeeds; only decoding detects missing packets at the end.
	if info, err := Probe(truncated); err != nil || info.Duration <= 0 || info.Width == 0 {
		t.Fatalf("fixture must pass metadata probe: %+v, %v", info, err)
	}
	if err := ValidateVideo(context.Background(), truncated); err == nil {
		t.Fatal("truncated video accepted despite decodable metadata")
	}
}

func TestValidateVideoRejectsEmptyDirectoryProtocolAndCancellation(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.mp4")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", empty, dir, "https://example.invalid/video.mp4", "pipe:0", "concat:a|b"} {
		if err := ValidateVideo(context.Background(), path); err == nil {
			t.Errorf("accepted invalid local file %q", path)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ValidateVideo(ctx, empty); !errors.Is(err, context.Canceled) {
		t.Errorf("cancellation not preserved: %v", err)
	}
}

func TestValidateVideoDoesNotFollowRemotePlaylist(t *testing.T) {
	requireValidationTools(t)
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "disguised.mp4")
	playlist := "#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXTINF:1,\n" + srv.URL + "/segment.ts\n#EXT-X-ENDLIST\n"
	if err := os.WriteFile(path, []byte(playlist), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateVideo(context.Background(), path); err == nil {
		t.Fatal("remote playlist accepted")
	}
	if requests.Load() != 0 {
		t.Fatalf("validation accessed network %d times", requests.Load())
	}
}
