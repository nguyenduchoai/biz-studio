package whisper

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

// Transcribe bóc băng file audio HOẶC video thành transcript có mốc TỪNG TỪ.
// lang rỗng → model tự nhận diện. upd nhận progress 0..100 + mô tả bước.
func Transcribe(ctx context.Context, st *store.Store, src, lang string,
	upd func(float64, string)) (*Transcript, error) {

	if upd == nil {
		upd = func(float64, string) {}
	}
	py := PythonPath(st)
	if py == "" {
		return nil, fmt.Errorf("%s", ErrChuaCai)
	}
	if _, err := os.Stat(src); err != nil {
		return nil, fmt.Errorf("không tìm thấy file cần bóc băng: %s", src)
	}
	runner, err := runnerPath(st)
	if err != nil {
		return nil, err
	}

	upd(3, "Chuẩn hoá âm thanh 16 kHz…")
	wav, cleanup, err := toWav16k(ctx, src)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	args := []string{runner,
		"--audio", wav,
		"--model", modelName(st),
		"--compute", computeType(st),
		"--model-dir", ModelDir(st),
	}
	if l := NormalizeLang(lang); l != "" {
		args = append(args, "--lang", l)
	}
	bin, argv := whisperArgv(py, args...)

	upd(6, fmt.Sprintf("Nạp model %s (lần đầu sẽ tải về, có thể mất vài phút)…", modelName(st)))
	tr, err := runStreaming(ctx, bin, argv, upd)
	if err != nil {
		return nil, err
	}
	if tr == nil || len(tr.Segments) == 0 {
		return nil, fmt.Errorf("faster-whisper không bóc được nội dung nào — kiểm tra file có tiếng nói không")
	}
	upd(98, fmt.Sprintf("Đã bóc %d đoạn · %d từ có mốc", len(tr.Segments), tr.WordCount()))
	return tr, nil
}

// toWav16k chuyển mọi định dạng (video/audio) về WAV mono 16 kHz trong thư mục
// tạm — faster-whisper đọc nhanh và ổn định nhất với dạng này.
func toWav16k(ctx context.Context, src string) (string, func(), error) {
	f, err := os.CreateTemp("", "bizstudio-whisper-*.wav")
	if err != nil {
		return "", func() {}, fmt.Errorf("tạo file audio tạm: %w", err)
	}
	dst := f.Name()
	f.Close()
	cleanup := func() { os.Remove(dst) }

	_, se, err := util.RunErr(ctx, "ffmpeg", "-y", "-hide_banner", "-i", src,
		"-vn", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", dst)
	if err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("tách âm thanh từ %s thất bại: %w — %s",
			filepath.Base(src), err, tailStr(se, 300))
	}
	return dst, cleanup, nil
}

// ndjsonLine — một dòng NDJSON runner python in ra.
type ndjsonLine struct {
	Type       string      `json:"type"`
	Model      string      `json:"model"`
	Language   string      `json:"language"`
	Duration   float64     `json:"duration"`
	Segment    *Segment    `json:"segment"`
	Transcript *Transcript `json:"transcript"`
}

// runStreaming chạy runner và đọc NDJSON từng dòng để báo tiến độ theo thời
// gian thực (video 10 phút mà im ru thì người dùng tưởng treo).
func runStreaming(ctx context.Context, bin string, argv []string,
	upd func(float64, string)) (*Transcript, error) {

	cmd := exec.CommandContext(ctx, bin, argv...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("không mở được luồng kết quả: %w", err)
	}
	var se bytes.Buffer
	cmd.Stderr = &se
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("không chạy được faster-whisper: %w", err)
	}

	acc := &Transcript{}
	var final *Transcript
	rd := bufio.NewReader(stdout)
	for {
		line, readErr := rd.ReadString('\n')
		if s := strings.TrimSpace(line); s != "" {
			applyLine(s, acc, &final, upd)
		}
		if readErr != nil {
			if readErr != io.EOF {
				_ = cmd.Wait()
				return nil, fmt.Errorf("đọc kết quả faster-whisper lỗi: %w", readErr)
			}
			break
		}
	}
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("bóc băng bị huỷ: %w", ctx.Err())
		}
		return nil, fmt.Errorf("faster-whisper lỗi: %w — %s", err, errHint(se.String()))
	}
	if final != nil && len(final.Segments) > 0 {
		return final, nil
	}
	if len(acc.Segments) > 0 {
		return acc, nil
	}
	return nil, nil
}

// applyLine xử lý 1 dòng NDJSON: cập nhật transcript tích luỹ + tiến độ.
func applyLine(s string, acc *Transcript, final **Transcript, upd func(float64, string)) {
	var ln ndjsonLine
	if err := json.Unmarshal([]byte(s), &ln); err != nil {
		return // dòng lạ (log của thư viện) — bỏ qua
	}
	switch ln.Type {
	case "info":
		acc.Language, acc.Duration = ln.Language, ln.Duration
		upd(10, fmt.Sprintf("Đang bóc băng (%s · %.1f giây)…", langLabel(ln.Language), ln.Duration))
	case "segment":
		if ln.Segment == nil {
			return
		}
		acc.Segments = append(acc.Segments, *ln.Segment)
		if acc.Duration > 0 {
			p := 10 + 85*ln.Segment.End/acc.Duration
			if p > 95 {
				p = 95
			}
			upd(p, fmt.Sprintf("Đoạn %d (%.1fs/%.1fs): %s",
				len(acc.Segments), ln.Segment.End, acc.Duration, shortRunes(ln.Segment.Text, 48)))
		} else {
			upd(50, fmt.Sprintf("Đoạn %d: %s", len(acc.Segments), shortRunes(ln.Segment.Text, 48)))
		}
	case "done":
		if ln.Transcript != nil {
			*final = ln.Transcript
		}
	}
}

func langLabel(code string) string {
	switch code {
	case "vi":
		return "tiếng Việt"
	case "en":
		return "tiếng Anh"
	case "":
		return "tự nhận diện"
	}
	return code
}

func shortRunes(v string, n int) string {
	v = strings.TrimSpace(v)
	r := []rune(v)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return v
}

func tailStr(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return "…" + s[len(s)-n:]
	}
	return s
}
