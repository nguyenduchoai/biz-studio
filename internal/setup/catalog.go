package setup

import "strings"

type pkgNames struct{ brew, brewCask, winget, apt, dnf, pacman string }

// Tool mô tả một công cụ cài được từ giao diện.
type Tool struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	Desc           string `json:"desc"`
	Manual         string `json:"manual"`
	Full           bool   `json:"full"`
	WindowsPackage string `json:"windowsPackage,omitempty"`

	pkg        pkgNames
	script     string
	selfUpdate []string
	aliases    []string
}

// Tools là catalog và thứ tự cài Full.
func Tools() []Tool {
	tools := []Tool{
		{ID: "git", Label: "Git for Windows", Desc: "Công cụ nền được Claude CLI và các quy trình dự án khuyên dùng.",
			Manual: "https://git-scm.com/downloads", Full: true,
			pkg: pkgNames{brew: "git", winget: "Git.Git", apt: "git", dnf: "git", pacman: "git"}},
		{ID: "python", Label: "Python 3.11", Desc: "Nền tảng để cài VieNeu-TTS và faster-whisper trên máy.",
			Manual: "https://www.python.org/downloads/", Full: true,
			pkg:     pkgNames{brew: "python@3.11", winget: "Python.Python.3.11", apt: "python3", dnf: "python3", pacman: "python"},
			aliases: []string{"python3", "py"}},
		{ID: "ffmpeg", Label: "FFmpeg", Desc: "Bộ xử lý video/âm thanh — gần như mọi tính năng đều cần.",
			Manual: "https://ffmpeg.org/download.html", Full: true,
			pkg: pkgNames{brew: "ffmpeg", winget: "Gyan.FFmpeg", apt: "ffmpeg", dnf: "ffmpeg", pacman: "ffmpeg"}},
		{ID: "ytdlp", Label: "yt-dlp", Desc: "Tải video về từ YouTube/TikTok… Bản cũ hay lỗi 403 — nên cập nhật thường xuyên.",
			Manual: "https://github.com/yt-dlp/yt-dlp#installation", Full: true,
			pkg:        pkgNames{brew: "yt-dlp", winget: "yt-dlp.yt-dlp", apt: "yt-dlp", dnf: "yt-dlp", pacman: "yt-dlp"},
			selfUpdate: []string{"yt-dlp", "-U"}},
		{ID: "chrome", Label: "Google Chrome", Desc: "Trình duyệt để render HTML Video.",
			Manual: "https://www.google.com/chrome/", Full: true,
			pkg: pkgNames{brewCask: "google-chrome", winget: "Google.Chrome"}, aliases: []string{"chromium"}},
		{ID: "claude", Label: "Claude CLI", Desc: "CLI chính thức cho Phiên AI, dịch thuật và tác vụ Claude.",
			Manual: "https://code.claude.com/docs/en/installation", Full: true,
			pkg:     pkgNames{brewCask: "claude-code", winget: "Anthropic.ClaudeCode"},
			aliases: []string{"claude cli", "claude code", "claudecode"}},
		{ID: "vieneu", Label: "VieNeu-TTS", Desc: "Giọng đọc tiếng Việt tự nhiên chạy ngay trên máy, không cần mạng.",
			Manual: "https://www.python.org/downloads/", Full: true,
			script: "setup-vieneu", aliases: []string{"tts", "giọng"}},
		{ID: "whisper", Label: "faster-whisper", Desc: "Bóc băng offline có mốc từng từ (cho phụ đề karaoke).",
			Manual: "https://pypi.org/project/faster-whisper/", Full: true,
			script: "setup-whisper", aliases: []string{"asr"}},
	}
	for i := range tools {
		tools[i].WindowsPackage = tools[i].pkg.winget
	}
	return tools
}

func Find(name string) (Tool, bool) {
	want := normalizeName(name)
	if want == "" {
		return Tool{}, false
	}
	for _, tool := range Tools() {
		if normalizeName(tool.ID) == want || normalizeName(tool.Label) == want {
			return tool, true
		}
		for _, alias := range tool.aliases {
			if normalizeName(alias) == want {
				return tool, true
			}
		}
	}
	return Tool{}, false
}

func normalizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if r != '-' && r != '_' && r != ' ' && r != '.' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
