package avatar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ---------- chế độ local: gọi thẳng torchrun trên máy có GPU ----------

// demoScript — script demo của LongCat cho nhánh một giọng.
const demoScript = "run_demo_avatar_single_audio_to_video.py"

// checkLocal kiểm các thứ cần có, KHÔNG chạy model.
func checkLocal(c cfg) (bool, string) {
	if c.repo == "" {
		return false, "Chưa khai thư mục mã nguồn LongCat-Video (cài bằng ./scripts/setup-longcat.sh)"
	}
	if !fileExists(filepath.Join(c.repo, demoScript)) {
		return false, "Thư mục mã nguồn không đúng — không thấy " + demoScript + " trong " + c.repo
	}
	if c.checkpoint == "" {
		return false, "Chưa khai thư mục trọng số model (weights/LongCat-Video-Avatar-1.5)"
	}
	if !dirExists(c.checkpoint) {
		return false, "Không thấy thư mục trọng số: " + c.checkpoint
	}
	py := pythonOf(c)
	if py == "" {
		return false, "Không tìm thấy python của môi trường LongCat"
	}
	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		return false, "Máy này không có GPU NVIDIA — LongCat không chạy được ở chế độ local. " +
			"Dùng chế độ remote để đẩy việc sang máy có GPU."
	}
	return true, fmt.Sprintf("Sẵn sàng · %d GPU%s · %s", c.gpus, int8Note(c.int8), filepath.Base(c.checkpoint))
}

func int8Note(on bool) string {
	if on {
		return " · nén INT8 (đỡ VRAM)"
	}
	return ""
}

// pythonOf tìm python: khai rõ thì dùng, không thì dò venv cạnh repo.
func pythonOf(c cfg) string {
	if c.python != "" {
		if fileExists(c.python) {
			return c.python
		}
		if p, err := exec.LookPath(c.python); err == nil {
			return p
		}
		return ""
	}
	if c.repo != "" {
		if p := filepath.Join(c.repo, "venv", "bin", "python"); fileExists(p) {
			return p
		}
	}
	if p, err := exec.LookPath("python3"); err == nil {
		return p
	}
	return ""
}

// inputSpec — đúng định dạng file JSON mà script demo của LongCat đọc.
type inputSpec struct {
	Prompt    string            `json:"prompt"`
	CondImage string            `json:"cond_image"`
	CondAudio map[string]string `json:"cond_audio"`
}

// writeInputJSON ghi file mô tả đầu vào. LongCat KHÔNG nhận ảnh/audio qua tham
// số dòng lệnh mà đọc từ một file JSON, nên phải sinh file này mỗi lần chạy.
func writeInputJSON(dir string, o Opts) (string, error) {
	spec := inputSpec{
		Prompt:    promptOf(o.Prompt),
		CondImage: o.ImagePath,
		CondAudio: map[string]string{"person1": o.AudioPath},
	}
	raw, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return "", fmt.Errorf("mã hoá mô tả đầu vào: %w", err)
	}
	p := filepath.Join(dir, "input.json")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		return "", fmt.Errorf("ghi mô tả đầu vào: %w", err)
	}
	return p, nil
}

// torchrunArgs dựng danh sách tham số cho script demo.
func torchrunArgs(c cfg, inputJSON, outDir string) []string {
	args := []string{
		"--nproc_per_node=" + strconv.Itoa(c.gpus),
		demoScript,
		"--checkpoint_dir=" + c.checkpoint,
		"--stage_1=at2v",
		"--input_json=" + inputJSON,
		"--use_distill",
		"--model_type", "avatar-v1.5",
		"--output_dir=" + outDir,
	}
	if c.gpus > 1 {
		// Chia model theo chiều ngữ cảnh khi có nhiều GPU — đúng cách repo hướng dẫn.
		args = append(args[:1], append([]string{"--context_parallel_size=" + strconv.Itoa(c.gpus)}, args[1:]...)...)
	}
	if c.int8 {
		args = append(args, "--use_int8")
	}
	return args
}

// generateLocal chạy model ngay trên máy này.
func generateLocal(ctx context.Context, c cfg, o Opts, dst string, upd func(float64, string)) error {
	if ok, why := checkLocal(c); !ok {
		return fmt.Errorf("%s", why)
	}
	work, err := os.MkdirTemp("", "longcat-*")
	if err != nil {
		return fmt.Errorf("tạo thư mục làm việc: %w", err)
	}
	defer os.RemoveAll(work)

	inputJSON, err := writeInputJSON(work, o)
	if err != nil {
		return err
	}
	outDir := filepath.Join(work, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("tạo thư mục kết quả: %w", err)
	}

	upd(10, fmt.Sprintf("Chạy LongCat trên %d GPU — thường mất vài phút…", c.gpus))
	cmd := exec.CommandContext(ctx, torchrunBin(c), torchrunArgs(c, inputJSON, outDir)...)
	cmd.Dir = c.repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("LongCat chạy thất bại: %w — %s", err, tailStr(string(out), 600))
	}

	upd(90, "Thu kết quả…")
	mp4, err := findVideo(outDir)
	if err != nil {
		return fmt.Errorf("%w — nhật ký: %s", err, tailStr(string(out), 400))
	}
	return moveFile(mp4, dst)
}

// torchrunBin — torchrun nằm cạnh python của venv.
func torchrunBin(c cfg) string {
	py := pythonOf(c)
	if py != "" {
		if t := filepath.Join(filepath.Dir(py), "torchrun"); fileExists(t) {
			return t
		}
	}
	return "torchrun"
}

// findVideo tìm file mp4 model vừa sinh ra (tên file do script demo đặt).
func findVideo(dir string) (string, error) {
	var found string
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || found != "" {
			return nil
		}
		if strings.EqualFold(filepath.Ext(p), ".mp4") {
			found = p
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("LongCat chạy xong nhưng không sinh ra file video nào")
	}
	return found, nil
}
