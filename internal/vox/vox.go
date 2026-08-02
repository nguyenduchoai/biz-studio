// Package vox — engine render "Bài viết → Video" / "Vox-Director":
// từ danh sách cảnh → video hoàn chỉnh (TTS + ảnh nền + phụ đề + nhạc nền) bằng ffmpeg.
package vox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"bizstudio/internal/media"
	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

// Scene — một cảnh trong video.
type Scene struct {
	Title        string  `json:"title"`
	VoiceText    string  `json:"voiceText"`
	MediaPath    string  `json:"mediaPath"`
	MediaKeyword string  `json:"mediaKeyword"`
	Duration     float64 `json:"duration"`
}

// Config — cấu hình render.
type Config struct {
	Aspect    string  `json:"aspect"` // "16:9" | "9:16" (mặc định "9:16")
	Voice     string  `json:"voice"`
	Engine    string  `json:"engine"`
	Theme     string  `json:"theme"`
	BgmPath   string  `json:"bgmPath"`
	BgmVolume float64 `json:"bgmVolume"` // 0..1, mặc định 0.25
	Quality   string  `json:"quality"`
	BurnSub   bool    `json:"burnSub"`
}

// Render dựng video hoàn chỉnh từ danh sách cảnh, trả đường dẫn tuyệt đối file mp4.
// upd nhận progress 0..100 + mô tả bước hiện tại. Khi lỗi, thư mục tmp/ được giữ
// nguyên để debug; chỉ dọn khi render thành công.
func Render(ctx context.Context, st *store.Store, scenes []Scene, cfg Config, workDir string, upd func(float64, string)) (string, error) {
	if len(scenes) == 0 {
		return "", errors.New("không có cảnh nào để render")
	}
	if upd == nil {
		upd = func(float64, string) {}
	}
	w, h, err := resolveSize(cfg.Aspect)
	if err != nil {
		return "", err
	}
	tmpDir := filepath.Join(workDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", fmt.Errorf("không tạo được thư mục làm việc %s: %w", tmpDir, err)
	}
	if findFont() == "" {
		st.AddLog("warn", "vox", "Không tìm thấy font hệ thống — tiêu đề cảnh sẽ không được vẽ lên video")
	}

	clips, durs, err := renderScenes(ctx, st, scenes, cfg, w, h, tmpDir, upd)
	if err != nil {
		return "", err
	}

	upd(72, "Ghép các cảnh…")
	merged := filepath.Join(tmpDir, "merged.mp4")
	if err := media.Concat(ctx, clips, merged); err != nil {
		return "", fmt.Errorf("ghép các cảnh thất bại: %w", err)
	}

	out, err := assembleFinal(ctx, merged, scenes, durs, cfg, workDir, tmpDir, upd)
	if err != nil {
		return "", err
	}

	upd(98, "Dọn dẹp file tạm…")
	_ = os.RemoveAll(tmpDir)

	abs, aerr := filepath.Abs(out)
	if aerr != nil {
		return out, nil
	}
	return abs, nil
}

// renderScenes dựng clip cho từng cảnh (TTS + ảnh nền + drawtext).
func renderScenes(ctx context.Context, st *store.Store, scenes []Scene, cfg Config, w, h int, tmpDir string, upd func(float64, string)) ([]string, []float64, error) {
	n := len(scenes)
	clips := make([]string, 0, n)
	durs := make([]float64, 0, n)
	for i, sc := range scenes {
		if err := ctx.Err(); err != nil {
			return nil, nil, fmt.Errorf("render bị hủy: %w", err)
		}
		upd(float64(i)/float64(n)*70, fmt.Sprintf("Cảnh %d/%d: %s", i+1, n, sceneLabel(sc)))
		clip, dur, err := buildScene(ctx, st, sc, i, cfg, w, h, tmpDir)
		if err != nil {
			return nil, nil, fmt.Errorf("cảnh %d (%s): %w", i+1, sceneLabel(sc), err)
		}
		clips = append(clips, clip)
		durs = append(durs, dur)
	}
	return clips, durs, nil
}

// assembleFinal trộn nhạc nền, sinh phụ đề SRT và xuất file cuối vox-final.mp4.
func assembleFinal(ctx context.Context, merged string, scenes []Scene, durs []float64, cfg Config, workDir, tmpDir string, upd func(float64, string)) (string, error) {
	cur := merged
	if cfg.BgmPath != "" {
		if !fileExists(cfg.BgmPath) {
			return "", fmt.Errorf("không tìm thấy file nhạc nền: %s", cfg.BgmPath)
		}
		upd(82, "Trộn nhạc nền…")
		withBgm := filepath.Join(tmpDir, "with-bgm.mp4")
		if err := mixBgm(ctx, cur, cfg.BgmPath, bgmVolume(cfg), withBgm); err != nil {
			return "", fmt.Errorf("trộn nhạc nền thất bại: %w", err)
		}
		cur = withBgm
	}

	final := filepath.Join(workDir, "vox-final.mp4")
	srtPath := filepath.Join(workDir, "vox-final.srt")
	upd(88, "Tạo phụ đề…")
	if err := writeSRT(scenes, durs, srtPath); err != nil {
		return "", fmt.Errorf("ghi phụ đề thất bại: %w", err)
	}

	if cfg.BurnSub {
		upd(92, "Gắn phụ đề vào video…")
		if err := media.BurnSubs(ctx, cur, srtPath, final); err != nil {
			return "", fmt.Errorf("gắn phụ đề thất bại: %w", err)
		}
		return final, nil
	}
	if err := moveFile(cur, final); err != nil {
		return "", fmt.Errorf("không ghi được file kết quả: %w", err)
	}
	return final, nil
}

// mixBgm loop nhạc nền vô hạn, hạ volume rồi amix với tiếng thuyết minh
// (duration=first: kết thúc theo video; normalize=0: giữ nguyên mức tiếng đọc).
func mixBgm(ctx context.Context, video, bgm string, vol float64, dst string) error {
	fc := fmt.Sprintf("[1:a]volume=%.3f[bg];[0:a][bg]amix=inputs=2:duration=first:dropout_transition=2:normalize=0[aout]", vol)
	_, err := util.Run(ctx, "ffmpeg",
		"-y",
		"-i", video,
		"-stream_loop", "-1", "-i", bgm,
		"-filter_complex", fc,
		"-map", "0:v", "-map", "[aout]",
		"-c:v", "copy",
		"-c:a", "aac", "-b:a", "192k",
		dst,
	)
	return err
}

func resolveSize(aspect string) (int, int, error) {
	switch aspect {
	case "16:9":
		return 1920, 1080, nil
	case "", "9:16":
		return 1080, 1920, nil
	default:
		return 0, 0, fmt.Errorf("tỉ lệ khung hình không hợp lệ: %q (chỉ hỗ trợ 16:9 hoặc 9:16)", aspect)
	}
}

func bgmVolume(cfg Config) float64 {
	if cfg.BgmVolume <= 0 || cfg.BgmVolume > 1 {
		return 0.25
	}
	return cfg.BgmVolume
}

func sceneLabel(sc Scene) string {
	if t := strings.TrimSpace(sc.Title); t != "" {
		return t
	}
	return "(không tiêu đề)"
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// moveFile đổi tên file, fallback copy nếu rename thất bại (khác filesystem).
func moveFile(src, dst string) error {
	_ = os.Remove(dst)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
