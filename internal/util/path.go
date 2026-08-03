package util

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AugmentPATH bổ sung các thư mục binary quen thuộc vào PATH của process.
// Cần thiết khi app được mở từ Finder/launchd (dmg/app bundle): macOS chỉ cấp
// PATH tối thiểu (/usr/bin:/bin:...) nên claude, ffmpeg, yt-dlp... không được tìm thấy.
func AugmentPATH() {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".local", "bin"),   // claude CLI (cài native)
		filepath.Join(home, "bin"),
		"/opt/homebrew/bin", "/opt/homebrew/sbin", // Homebrew Apple Silicon
		"/usr/local/bin", "/usr/local/sbin",       // Homebrew Intel / cài tay
		filepath.Join(home, ".bun", "bin"),
		filepath.Join(home, ".volta", "bin"),
		filepath.Join(home, "Library", "pnpm"),
		filepath.Join(home, ".claude", "local"), // claude local install
	}
	// nvm: thêm bản node mới nhất nếu có
	if matches, err := filepath.Glob(filepath.Join(home, ".nvm", "versions", "node", "*", "bin")); err == nil && len(matches) > 0 {
		sort.Strings(matches)
		candidates = append(candidates, matches[len(matches)-1])
	}

	cur := os.Getenv("PATH")
	seen := map[string]bool{}
	for _, p := range strings.Split(cur, string(os.PathListSeparator)) {
		seen[p] = true
	}
	add := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if c == "" || seen[c] {
			continue
		}
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			add = append(add, c)
			seen[c] = true
		}
	}
	if len(add) > 0 {
		_ = os.Setenv("PATH", cur+string(os.PathListSeparator)+strings.Join(add, string(os.PathListSeparator)))
	}
}
