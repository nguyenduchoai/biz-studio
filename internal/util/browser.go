package util

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// FindChromium dò trình duyệt Chromium có thể chạy headless/--app. Trên
// Windows phải dò Program Files và LocalAppData vì WinGet không thêm Chrome
// vào PATH; Edge có sẵn cũng dùng được cho renderer.
func FindChromium(configured string, includeEdge bool) string {
	for _, candidate := range chromiumCandidatesFor(runtime.GOOS, configured, includeEdge, os.Getenv) {
		if candidate == "" {
			continue
		}
		if filepath.IsAbs(candidate) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	return ""
}

func chromiumCandidatesFor(goos, configured string, includeEdge bool, getenv func(string) string) []string {
	if configured = strings.TrimSpace(configured); configured != "" {
		return []string{configured}
	}
	out := []string{}
	switch goos {
	case "darwin":
		out = append(out,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium")
		if includeEdge {
			out = append(out, "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge")
		}
	case "windows":
		for _, base := range []string{getenv("ProgramFiles"), getenv("ProgramFiles(x86)"), getenv("LOCALAPPDATA")} {
			if strings.TrimSpace(base) == "" {
				continue
			}
			out = append(out, filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"))
			if includeEdge {
				out = append(out, filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"))
			}
		}
	}
	out = append(out, "google-chrome", "google-chrome-stable", "chromium", "chromium-browser")
	if includeEdge {
		out = append(out, "microsoft-edge", "msedge")
	}
	return out
}
