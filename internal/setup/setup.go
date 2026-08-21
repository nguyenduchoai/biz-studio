// Package setup cài/cập nhật các công cụ ngoài mà Biz Studio phụ thuộc.
//
// Vì sao cần: người dùng gặp lỗi kiểu "HTTP Error 403: Forbidden" khi tải video
// và không có cách nào đoán ra nguyên nhân thật là yt-dlp cũ 6 tuần. Bắt họ mở
// terminal gõ brew/winget là mất luôn phần lớn người dùng. Nút "Cài" / "Cập
// nhật" ngay cạnh dòng trạng thái giải quyết đúng chỗ đó.
//
// Nguyên tắc: KHÔNG tải script lạ từ mạng về chạy. Chỉ gọi trình quản lý gói
// của hệ điều hành (brew / winget / apt…) và các script cài đặt nhúng sẵn
// trong binary — thứ người dùng đã tin khi tải chính phần mềm này.
package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Step là một lệnh trong quy trình cài. Nhiều bước chạy tuần tự, bước nào lỗi
// thì dừng — không chạy tiếp để tránh che mất lỗi gốc.
type Step struct {
	Label string
	Bin   string
	Args  []string
	Env   []string // thêm vào môi trường hiện tại, dạng "K=V"
}

// Plan là toàn bộ việc cần làm để cài (hoặc cập nhật) một công cụ trên MÁY NÀY.
// Dựng plan tách khỏi lúc chạy để giao diện xem trước được sẽ chạy lệnh gì.
type Plan struct {
	Tool    string   `json:"tool"`
	Action  string   `json:"action"` // "install" | "update"
	Steps   []Step   `json:"-"`
	Cmds    []string `json:"cmds"`   // dạng chữ để hiện cho người dùng
	Cleanup []string `json:"-"`      // file tạm cần xoá sau khi chạy
	Manual  string   `json:"manual"` // hướng dẫn tay khi không tự cài được
}

// pkgNames — tên gói của cùng một công cụ ở từng trình quản lý gói.
type pkgNames struct{ brew, brewCask, winget, apt, dnf, pacman string }

// Tool mô tả một công cụ cài được từ giao diện.
type Tool struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Desc   string `json:"desc"`
	Manual string `json:"manual"` // trang tải chính thức, hiện khi tự cài thất bại

	pkg    pkgNames
	script string // tên script nhúng (không đuôi), rỗng = cài bằng trình quản lý gói

	// selfUpdate — lệnh tự cập nhật của chính công cụ, dùng khi trình quản lý gói
	// KHÔNG phải nơi đã cài nó (tải binary rời, cài bằng pip…). Không có thì bỏ.
	selfUpdate []string

	// aliases — các cách gọi khác mà người dùng hay gõ.
	aliases []string
}

// Tools là danh mục công cụ cài được. Thứ tự này cũng là thứ tự hiện ra.
func Tools() []Tool {
	return []Tool{
		{
			ID: "ffmpeg", Label: "FFmpeg", Desc: "Bộ xử lý video/âm thanh — gần như mọi tính năng đều cần.",
			Manual: "https://ffmpeg.org/download.html",
			pkg:    pkgNames{brew: "ffmpeg", winget: "Gyan.FFmpeg", apt: "ffmpeg", dnf: "ffmpeg", pacman: "ffmpeg"},
		},
		{
			ID: "ytdlp", Label: "yt-dlp", Desc: "Tải video về từ YouTube/TikTok… Bản cũ hay lỗi 403 — nên cập nhật thường xuyên.",
			Manual:     "https://github.com/yt-dlp/yt-dlp#installation",
			pkg:        pkgNames{brew: "yt-dlp", winget: "yt-dlp.yt-dlp", apt: "yt-dlp", dnf: "yt-dlp", pacman: "yt-dlp"},
			selfUpdate: []string{"yt-dlp", "-U"},
		},
		{
			ID: "chrome", Label: "Google Chrome", Desc: "Trình duyệt để render HTML Video.",
			Manual:  "https://www.google.com/chrome/",
			pkg:     pkgNames{brewCask: "google-chrome", winget: "Google.Chrome"},
			aliases: []string{"chromium"},
		},
		{
			ID: "vieneu", Label: "VieNeu-TTS", Desc: "Giọng đọc tiếng Việt tự nhiên chạy ngay trên máy, không cần mạng.",
			Manual:  "https://www.python.org/downloads/",
			script:  "setup-vieneu",
			aliases: []string{"tts", "giọng"},
		},
		{
			ID: "whisper", Label: "faster-whisper", Desc: "Bóc băng offline có mốc từng từ (cho phụ đề karaoke).",
			Manual:  "https://www.python.org/downloads/",
			script:  "setup-whisper",
			aliases: []string{"asr"},
		},
	}
}

// Find trả công cụ theo ID, theo tên hiển thị, hoặc theo cách viết quen thuộc.
//
// ID nội bộ là "ytdlp" nhưng tên lệnh thật — và cái người ta gõ — là "yt-dlp".
// Bắt gõ đúng ID là bắt người dùng học một cái tên chỉ tồn tại trong mã nguồn.
func Find(name string) (Tool, bool) {
	want := normalizeName(name)
	if want == "" {
		return Tool{}, false
	}
	for _, t := range Tools() {
		if normalizeName(t.ID) == want || normalizeName(t.Label) == want {
			return t, true
		}
		for _, a := range t.aliases {
			if normalizeName(a) == want {
				return t, true
			}
		}
	}
	return Tool{}, false
}

// normalizeName bỏ hoa/thường, gạch nối, gạch dưới và khoảng trắng: "yt-dlp",
// "YT_DLP" và "ytdlp" là một.
func normalizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if r == '-' || r == '_' || r == ' ' || r == '.' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// BuildPlan dựng quy trình cài/cập nhật cho máy đang chạy.
//
// tmpDir dùng để ghi script nhúng ra đĩa (script không chạy được từ trong
// binary). dataDir là thư mục data của studio, truyền vào script làm tham số 1.
func BuildPlan(t Tool, action, dataDir, tmpDir string) (*Plan, error) {
	if action != "install" && action != "update" {
		return nil, fmt.Errorf("hành động không hợp lệ: %q", action)
	}
	p := &Plan{Tool: t.ID, Action: action, Manual: t.Manual}

	if t.script != "" {
		st, cleanup, err := scriptStep(t, action, dataDir, tmpDir)
		if err != nil {
			return nil, err
		}
		p.Steps = []Step{st}
		p.Cleanup = cleanup
	} else {
		st, err := pkgStep(t, action)
		if err != nil {
			return nil, err
		}
		p.Steps = []Step{st}
	}

	for _, s := range p.Steps {
		p.Cmds = append(p.Cmds, strings.TrimSpace(s.Bin+" "+strings.Join(s.Args, " ")))
	}
	return p, nil
}

// scriptStep ghi script nhúng ra file tạm rồi trả lệnh chạy nó.
//
// Cài lại và cập nhật là cùng một việc với script venv (pip install ghi đè), nên
// không phân biệt action ở đây.
func scriptStep(t Tool, _, dataDir, tmpDir string) (Step, []string, error) {
	ext, bin, pre := ".sh", "bash", []string(nil)
	if runtime.GOOS == "windows" {
		ext, bin = ".ps1", "powershell"
		pre = []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File"}
	}
	name := t.script + ext
	body, err := scriptFile(name)
	if err != nil {
		return Step{}, nil, err
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return Step{}, nil, fmt.Errorf("tạo thư mục tạm: %w", err)
	}
	path := filepath.Join(tmpDir, name)
	if err := os.WriteFile(path, body, 0o755); err != nil {
		return Step{}, nil, fmt.Errorf("ghi script cài đặt: %w", err)
	}
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		abs = dataDir
	}
	return Step{
		Label: "Chạy " + name,
		Bin:   bin,
		Args:  append(append([]string{}, pre...), path, abs),
	}, []string{path}, nil
}
