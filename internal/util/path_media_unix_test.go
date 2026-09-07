//go:build !windows

package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreferMediaBin(t *testing.T) {
	full := filepath.Join(t.TempDir(), "Full media dữ liệu")
	if err := os.MkdirAll(full, 0755); err != nil {
		t.Fatal(err)
	}
	current := "/usr/local/bin:/usr/bin"
	if got := preferMediaBin(current, full); got != current {
		t.Fatalf("must not select incomplete installation: %q", got)
	}
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if err := os.WriteFile(filepath.Join(full, name), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	want := full + ":" + current
	if got := preferMediaBin(current+":"+full, full); got != want {
		t.Fatalf("full build must precede existing regular build: %q", got)
	}
	if got := preferMediaBin(want, full); got != want || strings.Count(got, full) != 1 {
		t.Fatalf("repeated augmentation must be idempotent: %q", got)
	}
	if got := preferMediaBin("", full); got != full {
		t.Fatalf("empty PATH must not add current-directory lookup: %q", got)
	}
}
