// Package whisper — bóc băng offline bằng faster-whisper, có mốc thời gian
// TỪNG TỪ. Mốc từng từ là nền tảng để cắt khoảng lặng an toàn (không nuốt âm
// cuối tiếng Việt) và làm phụ đề karaoke.
//
// Cùng khuôn với internal/tts/vieneu.go: venv riêng, runner python nhúng trong
// binary rồi tự ghi ra đĩa, ép kiến trúc arm64 trên Apple Silicon.
package whisper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

// Runner Python nhúng vào binary — tự ghi ra data/whisper/runner.py khi cần,
// nhờ đó bản đóng gói (dmg/exe) không phụ thuộc thư mục scripts/.
// In NDJSON từng segment ra stdout để Go báo tiến độ khi file dài.
const whisperRunner = `#!/usr/bin/env python3
# Runner faster-whisper cho Biz Studio (tu sinh — dung sua tay).
import argparse, json, sys

def emit(obj):
    sys.stdout.write(json.dumps(obj, ensure_ascii=False) + "\n")
    sys.stdout.flush()

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--audio", required=True)
    ap.add_argument("--model", default="small")
    ap.add_argument("--compute", default="auto")
    ap.add_argument("--lang", default="")
    ap.add_argument("--model-dir", default="")
    ap.add_argument("--out", default="")
    args = ap.parse_args()

    from faster_whisper import WhisperModel

    kwargs = {"device": "auto", "compute_type": (args.compute or "auto")}
    if args.model_dir:
        kwargs["download_root"] = args.model_dir
    emit({"type": "load", "model": args.model})
    model = WhisperModel(args.model, **kwargs)

    segments, info = model.transcribe(
        args.audio,
        language=(args.lang or None),
        word_timestamps=True,
        vad_filter=True,
    )
    duration = float(getattr(info, "duration", 0) or 0)
    language = getattr(info, "language", "") or ""
    emit({"type": "info", "language": language, "duration": duration})

    out = []
    for i, seg in enumerate(segments):
        words = []
        for w in (getattr(seg, "words", None) or []):
            t = (w.word or "").strip()
            if not t:
                continue
            words.append({"text": t,
                          "start": round(float(w.start), 3),
                          "end": round(float(w.end), 3)})
        item = {"index": i,
                "start": round(float(seg.start), 3),
                "end": round(float(seg.end), 3),
                "text": (seg.text or "").strip(),
                "words": words}
        out.append(item)
        emit({"type": "segment", "segment": item})

    if out and duration <= 0:
        duration = out[-1]["end"]
    result = {"language": language, "duration": round(duration, 3), "segments": out}
    if args.out:
        with open(args.out, "w", encoding="utf-8") as f:
            json.dump(result, f, ensure_ascii=False)
    emit({"type": "done", "transcript": result})

if __name__ == "__main__":
    main()
`

// PythonPath trả python của venv faster-whisper: Settings().WhisperPython nếu
// hợp lệ, ngược lại <DataDir>/whisper/venv/bin/python3. Rỗng nếu chưa cài.
func PythonPath(st *store.Store) string {
	if p := strings.TrimSpace(st.Settings().WhisperPython); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, rel := range []string{
		filepath.Join("whisper", "venv", "bin", "python3"),
		filepath.Join("whisper", "venv", "bin", "python"),
		filepath.Join("whisper", "venv", "Scripts", "python.exe"),
	} {
		p := filepath.Join(st.DataDir, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Available — đã cài venv faster-whisper chưa.
func Available(st *store.Store) bool { return PythonPath(st) != "" }

// ErrChuaCai — thông báo chuẩn khi chưa cài venv.
const ErrChuaCai = "chưa cài faster-whisper — chạy scripts/setup-whisper.sh (hoặc xem hướng dẫn ở Cấu hình & API)"

var (
	archOnce  sync.Once
	archIsARM bool
)

// whisperArgv: trên máy Apple Silicon, ép Python chạy arm64 qua /usr/bin/arch.
// Cần thiết khi binary Biz Studio là x86_64 (Rosetta) — process con kế thừa
// kiến trúc x86_64 sẽ không nạp được ctranslate2/numpy bản arm64 trong venv.
func whisperArgv(py string, args ...string) (string, []string) {
	archOnce.Do(func() {
		if runtime.GOOS == "darwin" {
			out, _, err := util.RunErr(context.Background(), "sysctl", "-n", "hw.optional.arm64")
			archIsARM = err == nil && strings.TrimSpace(out) == "1"
		}
	})
	if archIsARM {
		return "/usr/bin/arch", append([]string{"-arm64", py}, args...)
	}
	return py, args
}

// Run chạy python của venv whisper với kiến trúc đúng của máy (dùng cho lệnh
// ngắn như kiểm tra phiên bản ở Cấu hình & API).
func Run(ctx context.Context, st *store.Store, args ...string) (string, error) {
	py := PythonPath(st)
	if py == "" {
		return "", fmt.Errorf("%s", ErrChuaCai)
	}
	bin, argv := whisperArgv(py, args...)
	return util.Run(ctx, bin, argv...)
}

// runnerPath ghi runner nhúng ra data/whisper/runner.py (nếu thiếu/lỗi thời).
func runnerPath(st *store.Store) (string, error) {
	dir := filepath.Join(st.DataDir, "whisper")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, "runner.py")
	if cur, err := os.ReadFile(p); err != nil || string(cur) != whisperRunner {
		if err := os.WriteFile(p, []byte(whisperRunner), 0o755); err != nil {
			return "", fmt.Errorf("ghi runner faster-whisper: %w", err)
		}
	}
	return p, nil
}

// ModelDir — nơi lưu model tải về (giữ cùng chỗ với dữ liệu app).
func ModelDir(st *store.Store) string {
	return filepath.Join(st.DataDir, "whisper", "models")
}

// modelName / computeType đọc từ Settings, có mặc định an toàn.
func modelName(st *store.Store) string {
	if m := strings.TrimSpace(st.Settings().WhisperModel); m != "" {
		return m
	}
	return "small"
}

func computeType(st *store.Store) string {
	if c := strings.TrimSpace(st.Settings().WhisperCompute); c != "" {
		return c
	}
	return "auto"
}

// NormalizeLang đổi tên ngôn ngữ người dùng nhập thành mã ISO cho whisper.
// Rỗng = để model tự nhận diện.
func NormalizeLang(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	switch s {
	case "", "auto", "tự động", "tu dong":
		return ""
	case "vi", "vie", "vietnamese", "tiếng việt", "tieng viet", "việt", "viet":
		return "vi"
	case "en", "eng", "english", "tiếng anh", "tieng anh", "anh":
		return "en"
	case "ja", "japanese", "tiếng nhật", "tieng nhat", "nhật", "nhat":
		return "ja"
	case "ko", "korean", "tiếng hàn", "tieng han", "hàn", "han":
		return "ko"
	case "zh", "chinese", "tiếng trung", "tieng trung", "trung":
		return "zh"
	}
	if len(s) == 2 {
		return s
	}
	return ""
}

// errHint rút gọn stderr của runner python: lấy vài dòng CUỐI (nơi chứa dòng
// lỗi thật của traceback), kèm gợi ý cho các lỗi hay gặp.
func errHint(stderr string) string {
	s := strings.TrimSpace(stderr)
	switch {
	case strings.Contains(s, "No module named 'faster_whisper"):
		return "venv thiếu gói faster-whisper — chạy: ./scripts/setup-whisper.sh"
	case strings.Contains(s, "ConnectionError"), strings.Contains(s, "Max retries exceeded"),
		strings.Contains(s, "Failed to resolve"):
		return "không tải được model (mất mạng?) — tải sẵn bằng: ./scripts/setup-whisper.sh"
	case strings.Contains(s, "efficient_conversion") || strings.Contains(s, "compute type"):
		return "compute_type không hợp lệ — đổi Cấu hình & API → Whisper compute về \"auto\" hoặc \"int8\""
	}
	var lines []string
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			lines = append(lines, t)
		}
	}
	if len(lines) > 3 {
		lines = lines[len(lines)-3:]
	}
	out := strings.Join(lines, " | ")
	if r := []rune(out); len(r) > 400 {
		out = "…" + string(r[len(r)-400:])
	}
	return out
}
