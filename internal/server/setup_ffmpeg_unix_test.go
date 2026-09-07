//go:build !windows

package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFFmpegReadinessRequiresActualSubtitleAndTextFilters(t *testing.T) {
	for _, full := range []bool{false, true} {
		t.Run(map[bool]string{false: "regular formula", true: "full formula"}[full], func(t *testing.T) {
			root := t.TempDir()
			filters := " .. scale V->V Scale video\\n"
			if full {
				filters += " .. subtitles V->V Render subtitles\\n .. ass V->V Render ASS\\n T. drawtext V->V Draw text\\n"
			}
			ffmpeg := "#!/bin/sh\nif [ \"$1\" = \"-version\" ]; then printf 'ffmpeg version 9.0.1\\n'; else printf '" + filters + "'; fi\n"
			for name, body := range map[string]string{"ffmpeg": ffmpeg, "ffprobe": "#!/bin/sh\nprintf 'ffprobe version 9.0.1\\n'\n"} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", root)
			status := checkFFmpeg(context.Background())
			if status.OK != full {
				t.Fatalf("FFmpeg ready=%v, want=%v: %s", status.OK, full, status.Detail)
			}
			if !full && !strings.Contains(status.Detail, "subtitles") {
				t.Fatalf("missing actionable filter diagnosis: %s", status.Detail)
			}
		})
	}
}
