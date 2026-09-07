package util

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var mediaPrefix = sync.OnceValue(func() string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	arm := runtime.GOARCH == "arm64"
	if !arm {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "/usr/sbin/sysctl", "-n", "hw.optional.arm64").Output()
		arm = err == nil && strings.TrimSpace(string(out)) == "1"
	}
	if arm {
		return "/opt/homebrew/opt/ffmpeg-full/bin"
	}
	return "/usr/local/opt/ffmpeg-full/bin"
})

func nativeFullMediaBin() string { return mediaPrefix() }

// Homebrew's full build is keg-only. Prefer it inside this process without
// relinking or overwriting the user's regular ffmpeg installation. Recheck
// files on each call so a just-completed install takes effect immediately.
func preferMediaBin(current, preferred string) string {
	if preferred == "" {
		return current
	}
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		fi, err := os.Stat(filepath.Join(preferred, name))
		if err != nil || fi.IsDir() || fi.Mode()&0111 == 0 {
			return current
		}
	}
	entries := []string{preferred}
	for _, entry := range filepath.SplitList(current) {
		if entry != preferred {
			entries = append(entries, entry)
		}
	}
	return strings.Join(entries, string(os.PathListSeparator))
}
