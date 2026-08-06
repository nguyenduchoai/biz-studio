// Package avatar — dựng video người nói từ MỘT tấm ảnh + MỘT file giọng, bằng
// LongCat-Video-Avatar (Meituan, giấy phép MIT).
//
// Vì sao tách làm hai chế độ chạy: LongCat là model 13,6 tỉ tham số, bắt buộc
// GPU NVIDIA (torch+cu124, flash_attn) — KHÔNG có bản cho Apple Silicon hay CPU.
// Trong khi Biz Studio phần lớn chạy trên máy cá nhân, nhiều máy là Mac. Nên:
//
//	local  — Biz Studio chạy ngay trên máy có GPU: gọi thẳng torchrun.
//	remote — Biz Studio chạy trên máy thường, đẩy việc sang một máy GPU đang
//	         chạy scripts/longcat-worker.py. Máy cá nhân là bàn điều khiển,
//	         máy GPU là xưởng render.
package avatar

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bizstudio/internal/store"
)

// Chế độ chạy.
const (
	ModeOff    = ""
	ModeLocal  = "local"
	ModeRemote = "remote"
)

// ErrOff — chưa bật engine avatar.
var ErrOff = errors.New("chưa bật engine avatar nói (Cấu hình & API) — cần một máy có GPU NVIDIA, " +
	"chạy ngay trên máy đó (local) hoặc trỏ sang máy GPU khác (remote)")

// Status — trạng thái engine, để giao diện nói rõ đang thiếu gì.
type Status struct {
	Mode       string `json:"mode"`
	Ready      bool   `json:"ready"`
	Detail     string `json:"detail"`
	Repo       string `json:"repo"`
	Checkpoint string `json:"checkpoint"`
	WorkerURL  string `json:"workerUrl"`
	GPUs       int    `json:"gpus"`
	Int8       bool   `json:"int8"`
}

// Opts — một lần dựng video.
type Opts struct {
	ImagePath string `json:"imagePath"` // ảnh nhân vật (tuyệt đối)
	AudioPath string `json:"audioPath"` // file giọng (tuyệt đối)
	Prompt    string `json:"prompt"`    // mô tả bối cảnh cho model
}

// cfgOf rút cấu hình avatar từ Settings.
type cfg struct {
	mode       string
	python     string
	repo       string
	checkpoint string
	worker     string
	gpus       int
	int8       bool
}

func cfgOf(st *store.Store) cfg {
	s := st.Settings()
	c := cfg{
		mode:       strings.ToLower(strings.TrimSpace(s.LongCatMode)),
		python:     strings.TrimSpace(s.LongCatPython),
		repo:       strings.TrimSpace(s.LongCatRepo),
		checkpoint: strings.TrimSpace(s.LongCatCheckpoint),
		worker:     strings.TrimRight(strings.TrimSpace(s.LongCatWorkerURL), "/"),
		gpus:       s.LongCatGPUs,
		int8:       s.LongCatInt8,
	}
	if c.gpus <= 0 {
		c.gpus = 1
	}
	return c
}

// Check trả trạng thái engine. Không gọi model, chỉ kiểm điều kiện — dùng cho
// giao diện nên phải nhanh và không bao giờ lỗi.
func Check(ctx context.Context, st *store.Store) Status {
	c := cfgOf(st)
	out := Status{
		Mode: c.mode, Repo: c.repo, Checkpoint: c.checkpoint,
		WorkerURL: c.worker, GPUs: c.gpus, Int8: c.int8,
	}
	switch c.mode {
	case ModeLocal:
		out.Ready, out.Detail = checkLocal(c)
	case ModeRemote:
		out.Ready, out.Detail = checkRemote(ctx, c)
	default:
		out.Detail = "Chưa bật. LongCat cần GPU NVIDIA — chọn \"local\" nếu Biz Studio đang chạy " +
			"trên máy có GPU, chọn \"remote\" nếu muốn đẩy việc sang một máy GPU khác."
	}
	return out
}

// Generate dựng video nói và ghi ra dst (.mp4).
func Generate(ctx context.Context, st *store.Store, o Opts, dst string, upd func(float64, string)) error {
	if upd == nil {
		upd = func(float64, string) {}
	}
	if err := validate(o); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("tạo thư mục kết quả: %w", err)
	}
	c := cfgOf(st)
	switch c.mode {
	case ModeLocal:
		return generateLocal(ctx, c, o, dst, upd)
	case ModeRemote:
		return generateRemote(ctx, c, o, dst, upd)
	default:
		return ErrOff
	}
}

// validate kiểm đầu vào TRƯỚC khi tốn vài phút chạy model.
func validate(o Opts) error {
	if strings.TrimSpace(o.ImagePath) == "" {
		return errors.New("chưa chọn ảnh nhân vật")
	}
	if strings.TrimSpace(o.AudioPath) == "" {
		return errors.New("chưa chọn file giọng đọc")
	}
	if st, err := os.Stat(o.ImagePath); err != nil || st.IsDir() {
		return fmt.Errorf("không đọc được ảnh nhân vật: %s", o.ImagePath)
	}
	if st, err := os.Stat(o.AudioPath); err != nil || st.IsDir() {
		return fmt.Errorf("không đọc được file giọng: %s", o.AudioPath)
	}
	return nil
}

// DefaultPrompt — mô tả bối cảnh mặc định khi người dùng để trống. LongCat cần
// một câu mô tả cảnh; để rỗng thì model tự bịa bối cảnh, hay ra kết quả lệch.
const DefaultPrompt = "A person speaking directly to the camera in a well-lit indoor setting, " +
	"natural facial expressions and lip movements synchronized with the speech, steady framing."

// promptOf lấy mô tả, rỗng thì dùng mặc định.
func promptOf(p string) string {
	if s := strings.TrimSpace(p); s != "" {
		return s
	}
	return DefaultPrompt
}
