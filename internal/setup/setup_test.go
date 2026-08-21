package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Script cài đặt phải nằm TRONG binary. Bản phát hành không đóng gói thư mục
// scripts/, nên nếu chỗ nhúng bị hỏng thì nút "Cài" chết ở đúng những máy cần
// nó nhất — máy người dùng cuối tải bản dmg/zip về, không phải máy lập trình.
func TestEmbeddedScriptsExistForEveryPlatform(t *testing.T) {
	for _, tool := range Tools() {
		if tool.script == "" {
			continue
		}
		for _, ext := range []string{".sh", ".ps1"} {
			name := tool.script + ext
			body, err := scriptFile(name)
			if err != nil {
				t.Errorf("%s: thiếu script nhúng %s: %v", tool.ID, name, err)
				continue
			}
			if len(body) < 200 {
				t.Errorf("%s: script %s chỉ có %d byte — nghi là file rỗng", tool.ID, name, len(body))
			}
		}
	}
}

// Script .sh đã bị bỏ dòng cd theo vị trí file (giờ nó chạy từ thư mục tạm).
// Nếu ai đó thêm lại, thư mục data sẽ trỏ vào internal/setup và venv mọc ra sai
// chỗ — hỏng lặng lẽ, chỉ lộ ra khi người dùng bấm nút.
func TestShellScriptsDoNotCdRelativeToThemselves(t *testing.T) {
	for _, tool := range Tools() {
		if tool.script == "" {
			continue
		}
		body, err := scriptFile(tool.script + ".sh")
		if err != nil {
			t.Fatalf("%s: %v", tool.ID, err)
		}
		if strings.Contains(string(body), `cd "$(dirname "$0")`) {
			t.Errorf("%s.sh: còn cd theo vị trí file — sẽ tính sai thư mục data khi chạy từ binary", tool.script)
		}
	}
}

// Script cài venv PHẢI ép arm64 trên Apple Silicon.
//
// Đo thật: với binary Biz Studio bản x86_64 (chạy qua Rosetta) trên máy M-series,
// pip tải về wheel `macosx_11_0_x86_64`; nhưng lúc chạy, internal/whisper và
// internal/tts luôn gọi python qua `arch -arm64`. Kết quả là ImportError
// "incompatible architecture" ở tận bước nạp thư viện — chẳng nói gì tới việc
// cài, nên gần như không thể lần ra. Sau khi ép, wheel là `macosx_14_0_arm64`
// và cùng binary x86_64 đó cài xong chạy được.
//
// Phải dùng sysctl: dưới Rosetta, `uname -m` trả về x86_64 nên vô dụng.
func TestVenvScriptsForceNativeArchOnAppleSilicon(t *testing.T) {
	for _, tool := range Tools() {
		if tool.script == "" {
			continue
		}
		body, err := scriptFile(tool.script + ".sh")
		if err != nil {
			t.Fatalf("%s: %v", tool.ID, err)
		}
		s := string(body)
		if !strings.Contains(s, "hw.optional.arm64") {
			t.Errorf("%s.sh: không dò Apple Silicon bằng sysctl hw.optional.arm64", tool.script)
		}
		if !strings.Contains(s, "/usr/bin/arch -arm64") {
			t.Errorf("%s.sh: không ép arm64 — venv sẽ lệch kiến trúc khi app chạy Rosetta", tool.script)
		}
		// bash 3.2 của macOS báo "unbound variable" khi bung mảng rỗng dưới
		// `set -u`, nên prefix phải là chuỗi.
		if strings.Contains(s, `"${ARCH_PREFIX[@]}"`) {
			t.Errorf("%s.sh: ARCH_PREFIX dạng mảng sẽ làm script chết trên bash 3.2 khi máy không phải Apple Silicon", tool.script)
		}
		for _, must := range []string{"$ARCH_PREFIX python3 -m venv", `$ARCH_PREFIX "$VENV/bin/pip"`} {
			if !strings.Contains(s, must) {
				t.Errorf("%s.sh: thiếu %q — bước này chạy sai kiến trúc là hỏng cả venv", tool.script, must)
			}
		}
	}
}

// BuildPlan phải ghi script ra đĩa và truyền thư mục data dạng TUYỆT ĐỐI: tiến
// trình con thừa hưởng thư mục làm việc của server, không phải của script.
func TestBuildPlanScriptWritesFileAndPassesAbsoluteDataDir(t *testing.T) {
	tool, ok := Find("whisper")
	if !ok {
		t.Fatal("không tìm thấy công cụ whisper")
	}
	tmp := t.TempDir()
	plan, err := BuildPlan(tool, "install", "data", tmp)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("muốn 1 bước, có %d", len(plan.Steps))
	}
	args := plan.Steps[0].Args
	scriptPath, dataArg := args[len(args)-2], args[len(args)-1]

	if _, err := os.Stat(scriptPath); err != nil {
		t.Errorf("script chưa được ghi ra đĩa: %v", err)
	}
	if filepath.Dir(scriptPath) != tmp {
		t.Errorf("script ghi vào %s, muốn trong %s", scriptPath, tmp)
	}
	if !filepath.IsAbs(dataArg) {
		t.Errorf("thư mục data %q không phải đường dẫn tuyệt đối", dataArg)
	}
	if len(plan.Cleanup) != 1 || plan.Cleanup[0] != scriptPath {
		t.Errorf("Cleanup = %v, muốn [%s] — không dọn thì file tạm chất đống", plan.Cleanup, scriptPath)
	}
	if len(plan.Cmds) != 1 || !strings.Contains(plan.Cmds[0], "setup-whisper") {
		t.Errorf("Cmds = %v, muốn có setup-whisper để hiện cho người dùng", plan.Cmds)
	}
}

func TestBuildPlanRejectsUnknownAction(t *testing.T) {
	tool, _ := Find("ffmpeg")
	if _, err := BuildPlan(tool, "xoá", t.TempDir(), t.TempDir()); err == nil {
		t.Fatal("muốn lỗi với hành động lạ, nhưng không có")
	}
}

// Mỗi công cụ phải có đường lui thủ công: khi máy thiếu brew/winget, câu duy
// nhất còn giúp được người dùng là "tải ở đây".
func TestEveryToolHasManualFallbackURL(t *testing.T) {
	for _, tool := range Tools() {
		if !strings.HasPrefix(tool.Manual, "https://") {
			t.Errorf("%s: Manual = %q, cần một URL tải chính thức", tool.ID, tool.Manual)
		}
		if strings.TrimSpace(tool.Label) == "" || strings.TrimSpace(tool.Desc) == "" {
			t.Errorf("%s: thiếu Label hoặc Desc", tool.ID)
		}
	}
}

func TestFindUnknownTool(t *testing.T) {
	for _, name := range []string{"khong-co-that", "", "   ", "-"} {
		if _, ok := Find(name); ok {
			t.Errorf("Find(%q) trả true cho công cụ không tồn tại", name)
		}
	}
}

// ID nội bộ là "ytdlp" nhưng lệnh thật tên "yt-dlp" — và đó là cái người dùng
// gõ, cũng là cái hiện trong thông báo lỗi của chính yt-dlp. Bắt gõ đúng ID nội
// bộ là bắt học một cái tên chỉ có trong mã nguồn.
func TestFindAcceptsNamesPeopleActuallyType(t *testing.T) {
	cases := map[string]string{
		"ytdlp":          "ytdlp",
		"yt-dlp":         "ytdlp",
		"YT-DLP":         "ytdlp",
		"yt_dlp":         "ytdlp",
		"ffmpeg":         "ffmpeg",
		"FFmpeg":         "ffmpeg",
		"whisper":        "whisper",
		"faster-whisper": "whisper",
		"fasterwhisper":  "whisper",
		"asr":            "whisper",
		"vieneu":         "vieneu",
		"VieNeu-TTS":     "vieneu",
		"tts":            "vieneu",
		"chrome":         "chrome",
		"Google Chrome":  "chrome",
		"google-chrome":  "chrome",
		"chromium":       "chrome",
	}
	for name, wantID := range cases {
		got, ok := Find(name)
		if !ok {
			t.Errorf("Find(%q) không tìm thấy gì, muốn %s", name, wantID)
			continue
		}
		if got.ID != wantID {
			t.Errorf("Find(%q) = %s, muốn %s", name, got.ID, wantID)
		}
	}
}

// Hai công cụ trùng bí danh thì Find trả cái nào là chuyện may rủi theo thứ tự
// khai báo — bấm "Cài whisper" ra VieNeu là kiểu lỗi không ai ngờ tới.
func TestNoDuplicateNamesAcrossTools(t *testing.T) {
	seen := map[string]string{}
	for _, tool := range Tools() {
		names := append([]string{tool.ID, tool.Label}, tool.aliases...)
		for _, n := range names {
			k := normalizeName(n)
			// Cùng một công cụ trùng với chính nó là bình thường: ID "ffmpeg" và
			// nhãn "FFmpeg" chuẩn hoá ra một. Chỉ hai công cụ KHÁC nhau mới hỏng.
			if prev, dup := seen[k]; dup && prev != tool.ID {
				t.Errorf("tên %q dùng cho cả %s lẫn %s", n, prev, tool.ID)
			}
			seen[k] = tool.ID
		}
	}
}
