package util

import (
	"strings"
	"testing"
)

func TestWindowsChromiumCandidatesIncludeInstalledLocations(t *testing.T) {
	env := map[string]string{
		"ProgramFiles":      `C:\\Program Files`,
		"ProgramFiles(x86)": `C:\\Program Files (x86)`,
		"LOCALAPPDATA":      `C:\\Users\\An\\AppData\\Local`,
	}
	got := strings.Join(chromiumCandidatesFor("windows", "", true, func(k string) string { return env[k] }), "\n")
	for _, part := range []string{"Google", "Chrome", "chrome.exe", "Microsoft", "Edge", "msedge.exe"} {
		if !strings.Contains(got, part) {
			t.Fatalf("danh sách Windows thiếu %q:\n%s", part, got)
		}
	}
}
