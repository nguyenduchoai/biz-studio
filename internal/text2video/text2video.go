// Package text2video — engine Text → Video: bóc nội dung bài viết từ link, viết
// kịch bản LỜI ĐỌC bằng LLM, tổng hợp giọng đọc (đo thời lượng THẬT từng đoạn)
// rồi dựng video bám đúng giọng đã đọc.
//
// Mỗi phiên làm việc có một thư mục dữ liệu riêng: data/text2video/<sessionID>/
// chứa seg-<i>.wav (từng đoạn), voice.wav (giọng đọc đã ghép), transcript.json
// (mốc thời gian từng đoạn) và text2video.mp4 (video kết quả chế độ HTML).
package text2video

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"bizstudio/internal/util"
)

const (
	// DirName — thư mục chứa dữ liệu mọi phiên, nằm trong data/.
	DirName = "text2video"
	// VoiceFile — file giọng đọc đã ghép của phiên.
	VoiceFile = "voice.wav"
	// TranscriptFile — mốc thời gian từng đoạn lời đọc.
	TranscriptFile = "transcript.json"
	// VideoFile — video kết quả khi dựng bằng HTML Video.
	VideoFile = "text2video.mp4"

	// audioRate — sample rate chuẩn hoá mọi đoạn wav trước khi ghép.
	audioRate = 44100
	// charsPerSecond — ước lượng tốc độ đọc tiếng Việt (canh độ dài kịch bản).
	charsPerSecond = 15
)

// SessionDir trả thư mục dữ liệu của phiên: <dataDir>/text2video/<sessionID>.
func SessionDir(dataDir, sessionID string) string {
	return filepath.Join(dataDir, DirName, sessionID)
}

// dataRelPath trả đường dẫn file trong thư mục phiên theo dạng tương đối DataDir
// ("text2video/<id>/<name>" — FE mở qua /data/…). workDir không theo chuẩn (thư
// mục tạm khi test) → trả đường dẫn tuyệt đối để vẫn dùng được.
func dataRelPath(workDir, sessionID, name string) string {
	if filepath.Base(workDir) == sessionID && filepath.Base(filepath.Dir(workDir)) == DirName {
		return path.Join(DirName, sessionID, name)
	}
	if abs, err := filepath.Abs(filepath.Join(workDir, name)); err == nil {
		return abs
	}
	return filepath.Join(workDir, name)
}

// absFromData đổi đường dẫn tương đối DataDir thành tuyệt đối; đã tuyệt đối thì giữ nguyên.
func absFromData(dataDir, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(dataDir, filepath.FromSlash(p))
}

// runFFmpeg chạy ffmpeg, lỗi kèm phần cuối stderr (lỗi thật của ffmpeg nằm ở cuối).
func runFFmpeg(ctx context.Context, args ...string) error {
	if _, se, err := util.RunErr(ctx, "ffmpeg", args...); err != nil {
		return fmt.Errorf("ffmpeg lỗi: %w — %s", err, tail(se, 400))
	}
	return nil
}

// normalizeAudio chuyển một đoạn audio về wav mono 44100 (chuẩn chung để ghép).
func normalizeAudio(ctx context.Context, src, dst string) error {
	return runFFmpeg(ctx, "-y", "-i", src, "-vn",
		"-ar", fmt.Sprint(audioRate), "-ac", "1", "-c:a", "pcm_s16le", dst)
}

// concatAudio ghép tuần tự các file wav cùng chuẩn bằng concat demuxer.
func concatAudio(ctx context.Context, parts []string, tmpDir, dst string) error {
	if len(parts) == 0 {
		return fmt.Errorf("không có đoạn audio nào để ghép")
	}
	list, err := writeConcatList(parts, tmpDir)
	if err != nil {
		return err
	}
	if err := runFFmpeg(ctx, "-y", "-f", "concat", "-safe", "0", "-i", list,
		"-ar", fmt.Sprint(audioRate), "-ac", "1", "-c:a", "pcm_s16le", dst); err != nil {
		return fmt.Errorf("ghép giọng đọc thất bại: %w", err)
	}
	return nil
}

// writeConcatList ghi file danh sách cho concat demuxer (đường dẫn tuyệt đối,
// escape dấu nháy đơn).
func writeConcatList(parts []string, dir string) (string, error) {
	var b strings.Builder
	for _, p := range parts {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		fmt.Fprintf(&b, "file '%s'\n", strings.ReplaceAll(abs, "'", `'\''`))
	}
	list := filepath.Join(dir, "concat-voice.txt")
	if err := os.WriteFile(list, []byte(b.String()), 0o644); err != nil {
		return "", fmt.Errorf("không ghi được danh sách ghép %s: %w", list, err)
	}
	return list, nil
}

// copyFile sao chép file src → dst (tạo thư mục cha nếu cần).
func copyFile(src, dst string) error {
	if dir := filepath.Dir(dst); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("tạo thư mục %s: %w", dir, err)
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("mở file %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("tạo file %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("ghi file %s: %w", dst, err)
	}
	return out.Close()
}

// tail lấy tối đa n ký tự cuối của chuỗi (đã trim).
func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return "…" + s[len(s)-n:]
	}
	return s
}

// shortText rút gọn chuỗi hiển thị theo rune (an toàn tiếng Việt).
func shortText(v string, n int) string {
	v = strings.TrimSpace(v)
	r := []rune(v)
	if len(r) > n {
		return strings.TrimSpace(string(r[:n])) + "…"
	}
	return v
}
