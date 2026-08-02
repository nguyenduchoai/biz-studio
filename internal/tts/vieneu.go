package tts

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

// Runner Python nhúng vào binary — tự ghi ra data/vieneu/runner.py khi cần,
// nhờ đó bản đóng gói (dmg/exe) không phụ thuộc thư mục scripts/.
const vieneuRunner = `#!/usr/bin/env python3
# Runner VieNeu-TTS cho Biz Studio (tu sinh — dung sua tay).
import argparse, json, sys

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--list-voices", action="store_true")
    ap.add_argument("--text-file")
    ap.add_argument("--voice", default="")
    ap.add_argument("--style", default="tu_nhien")
    ap.add_argument("--out")
    args = ap.parse_args()

    from vieneu import Vieneu
    v = Vieneu()

    if args.list_voices:
        out = [{"id": vid, "label": label} for label, vid in v.list_preset_voices()]
        print(json.dumps(out, ensure_ascii=False))
        return

    with open(args.text_file, encoding="utf-8") as f:
        text = f.read()
    kwargs = {}
    if args.voice:
        kwargs["voice"] = args.voice
    if args.style:
        kwargs["style"] = args.style
    audio = v.infer(text, **kwargs)
    v.save(audio, args.out)
    print("OK", args.out)

if __name__ == "__main__":
    main()
`

// VieNeuPythonPath trả python của venv VieNeu: Settings().VieNeuPython nếu hợp lệ,
// ngược lại <DataDir>/vieneu/venv/bin/python3. Rỗng nếu chưa cài.
func VieNeuPythonPath(st *store.Store) string {
	if p := strings.TrimSpace(st.Settings().VieNeuPython); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, rel := range []string{
		filepath.Join("vieneu", "venv", "bin", "python3"),
		filepath.Join("vieneu", "venv", "bin", "python"),
		filepath.Join("vieneu", "venv", "Scripts", "python.exe"),
	} {
		p := filepath.Join(st.DataDir, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// VieNeuAvailable — đã cài venv VieNeu chưa.
func VieNeuAvailable(st *store.Store) bool { return VieNeuPythonPath(st) != "" }

var (
	archOnce  sync.Once
	archIsARM bool
)

// vieneuArgv: trên máy Apple Silicon, ép Python chạy arm64 qua /usr/bin/arch.
// Cần thiết khi binary Biz Studio là x86_64 (Rosetta) — process con kế thừa
// kiến trúc x86_64 sẽ không nạp được numpy/onnxruntime bản arm64 trong venv.
func vieneuArgv(py string, args ...string) (string, []string) {
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

// VieNeuRun chạy python của venv VieNeu với kiến trúc đúng của máy.
func VieNeuRun(ctx context.Context, st *store.Store, args ...string) (string, error) {
	py := VieNeuPythonPath(st)
	if py == "" {
		return "", fmt.Errorf("chưa cài VieNeu-TTS — chạy scripts/setup-vieneu.sh")
	}
	bin, argv := vieneuArgv(py, args...)
	return util.Run(ctx, bin, argv...)
}

// vieneuRunnerPath ghi runner nhúng ra data/vieneu/runner.py (nếu thiếu/lỗi thời).
func vieneuRunnerPath(st *store.Store) (string, error) {
	dir := filepath.Join(st.DataDir, "vieneu")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, "runner.py")
	if cur, err := os.ReadFile(p); err != nil || string(cur) != vieneuRunner {
		if err := os.WriteFile(p, []byte(vieneuRunner), 0o755); err != nil {
			return "", fmt.Errorf("ghi runner VieNeu: %w", err)
		}
	}
	return p, nil
}

// speakVieNeu tổng hợp giọng bằng VieNeu-TTS (48 kHz, on-device).
// voiceID hỗ trợ dạng "Tên giọng@phong_cách" (tu_nhien | tin_tuc | doc_truyen).
func speakVieNeu(ctx context.Context, st *store.Store, text, voiceID, dst string) error {
	py := VieNeuPythonPath(st)
	if py == "" {
		return fmt.Errorf("chưa cài VieNeu-TTS — chạy scripts/setup-vieneu.sh (hoặc xem hướng dẫn ở Cấu hình & API)")
	}
	runner, err := vieneuRunnerPath(st)
	if err != nil {
		return err
	}

	voice, style := voiceID, "tu_nhien"
	if i := strings.LastIndex(voiceID, "@"); i >= 0 {
		voice, style = voiceID[:i], voiceID[i+1:]
	}

	tmpTxt, err := os.CreateTemp("", "bizstudio-vieneu-*.txt")
	if err != nil {
		return fmt.Errorf("tạo file text tạm: %w", err)
	}
	defer os.Remove(tmpTxt.Name())
	if _, err := tmpTxt.WriteString(text); err != nil {
		tmpTxt.Close()
		return err
	}
	tmpTxt.Close()

	outWav := dst
	needConvert := !strings.EqualFold(filepath.Ext(dst), ".wav")
	if needConvert {
		outWav = dst + ".vieneu.wav"
		defer os.Remove(outWav)
	}

	args := []string{runner, "--text-file", tmpTxt.Name(), "--out", outWav, "--style", style}
	if strings.TrimSpace(voice) != "" {
		args = append(args, "--voice", voice)
	}
	bin, argv := vieneuArgv(py, args...)
	if _, err := util.Run(ctx, bin, argv...); err != nil {
		return fmt.Errorf("VieNeu-TTS lỗi: %w", err)
	}
	if _, err := os.Stat(outWav); err != nil {
		return fmt.Errorf("VieNeu-TTS không tạo được file audio")
	}
	if needConvert {
		if _, err := util.Run(ctx, "ffmpeg", "-y", "-i", outWav, dst); err != nil {
			return fmt.Errorf("chuyển đổi định dạng audio: %w", err)
		}
	}
	return nil
}

// vieneuVoices đọc danh sách giọng preset từ data/vieneu/voices.json
// (setup script sinh ra); fallback bộ tên đã biết nếu chưa có file.
func vieneuVoices(st *store.Store) []Voice {
	if !VieNeuAvailable(st) {
		return nil
	}
	fallback := []string{"Phạm Tuyên", "Minh Đức", "Trúc Ly", "Quang Sơn", "Ngọc Trân"}
	var out []Voice
	if b, err := os.ReadFile(filepath.Join(st.DataDir, "vieneu", "voices.json")); err == nil {
		var list []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		}
		if json.Unmarshal(b, &list) == nil && len(list) > 0 {
			for _, v := range list {
				// ID là tên giọng để gọi infer(); label dạng
				// "Minh Đức — Nam · Bắc · Phong cách tin tức" → tách giới tính + vùng miền.
				vc := Voice{ID: v.ID, Name: v.ID, Lang: "vi-VN", Engine: "vieneu"}
				if strings.Contains(v.Label, "Nữ") {
					vc.Gender = "Nữ"
				} else if strings.Contains(v.Label, "Nam ") || strings.Contains(v.Label, "· Nam") || strings.Contains(v.Label, "— Nam") {
					vc.Gender = "Nam"
				}
				for _, region := range []string{"Bắc", "Trung", "Nam Bộ", "Miền Nam"} {
					if strings.Contains(v.Label, "· "+region) {
						vc.Lang = "vi-VN · " + region
						break
					}
				}
				out = append(out, vc)
			}
			return out
		}
	}
	for _, n := range fallback {
		out = append(out, Voice{ID: n, Name: n, Lang: "vi-VN", Engine: "vieneu"})
	}
	return out
}
