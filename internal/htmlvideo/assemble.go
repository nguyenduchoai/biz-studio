package htmlvideo

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"bizstudio/internal/media"
	"bizstudio/internal/util"
)

// buildClips dựng clip MP4 từng cảnh từ chuỗi frame PNG + audio (70→86%).
func buildClips(ctx context.Context, jobs []*sceneJob, fps int, tmpDir string, upd func(float64, string)) ([]string, error) {
	clips := make([]string, 0, len(jobs))
	for i, j := range jobs {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("render bị hủy: %w", err)
		}
		upd(70+float64(i)/float64(len(jobs))*16, fmt.Sprintf("Dựng clip cảnh %d/%d…", i+1, len(jobs)))
		clip := filepath.Join(tmpDir, fmt.Sprintf("clip-%d.mp4", i))
		if err := encodeClip(ctx, j, fps, clip); err != nil {
			return nil, fmt.Errorf("cảnh %d (%s): %w", i+1, sceneLabel(j.scene), err)
		}
		clips = append(clips, clip)
	}
	return clips, nil
}

// encodeClip: ffmpeg -framerate FPS -i frame-%05d.png (+ wav | anullsrc)
// → libx264 yuv420p + aac. Mọi clip đều có track audio để concat copy được.
func encodeClip(ctx context.Context, j *sceneJob, fps int, dst string) error {
	args := []string{"-y",
		"-framerate", strconv.Itoa(fps),
		"-i", filepath.Join(j.frameDir, "frame-%05d.png"),
	}
	if j.wavPath != "" {
		args = append(args, "-i", j.wavPath, "-af", "apad") // đệm im lặng nếu audio ngắn hơn video
	} else {
		args = append(args, "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo")
	}
	args = append(args,
		"-t", fmt.Sprintf("%.3f", j.durS),
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-crf", "20", "-preset", "veryfast",
		"-c:a", "aac", "-b:a", "192k",
		dst,
	)
	if _, err := util.Run(ctx, "ffmpeg", args...); err != nil {
		return fmt.Errorf("ffmpeg dựng clip thất bại: %w", err)
	}
	return nil
}

// assembleFinal ghép các clip, trộn nhạc nền, sinh SRT cạnh output và
// (tuỳ chọn) gắn cứng phụ đề → workDir/htmlvideo-final.mp4 (86→98%).
func assembleFinal(ctx context.Context, clips []string, jobs []*sceneJob, cfg Config, workDir, tmpDir string, upd func(float64, string)) (string, error) {
	upd(87, "Ghép các cảnh…")
	merged := filepath.Join(tmpDir, "merged.mp4")
	if err := media.Concat(ctx, clips, merged); err != nil {
		return "", fmt.Errorf("ghép các cảnh thất bại: %w", err)
	}

	cur := merged
	if bgm := strings.TrimSpace(cfg.BgmPath); bgm != "" {
		if !fileExists(bgm) {
			return "", fmt.Errorf("không tìm thấy file nhạc nền: %s", bgm)
		}
		upd(90, "Trộn nhạc nền…")
		withBgm := filepath.Join(tmpDir, "with-bgm.mp4")
		if err := mixBgm(ctx, cur, bgm, cfg.bgmVolume(), withBgm); err != nil {
			return "", fmt.Errorf("trộn nhạc nền thất bại: %w", err)
		}
		cur = withBgm
	}

	final := filepath.Join(workDir, "htmlvideo-final.mp4")
	srtPath := filepath.Join(workDir, "htmlvideo-final.srt")
	upd(93, "Tạo phụ đề…")
	if err := writeSRT(jobs, srtPath); err != nil {
		return "", fmt.Errorf("ghi phụ đề thất bại: %w", err)
	}

	if cfg.BurnSub {
		upd(95, "Gắn phụ đề vào video…")
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

// mixBgm loop nhạc nền vô hạn, hạ volume rồi amix với thuyết minh
// (duration=first: kết thúc theo video; normalize=0: giữ mức tiếng đọc).
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
