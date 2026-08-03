package dubbing

import (
	"context"
	"fmt"

	"bizstudio/internal/media"
	"bizstudio/internal/store"
)

// muxVideo ghép track lồng tiếng vào video, giữ nguyên hình (-c:v copy).
//
// KeepOriginal: hạ âm lượng tiếng gốc rồi amix với track lồng tiếng
// (duration=first → kết thúc theo tiếng gốc, tức theo độ dài video).
// Ngược lại chỉ dùng track lồng tiếng, apad + -shortest để tiếng phủ đúng
// độ dài video (không cắt cụt hình khi track ngắn hơn).
func muxVideo(ctx context.Context, st *store.Store, videoPath, dubWav, dst string, cfg Config) error {
	keep := cfg.KeepOriginal
	if keep && !media.HasAudio(ctx, videoPath) {
		keep = false
		st.AddLog("warn", "dubbing", "Video gốc không có tiếng — bỏ qua tuỳ chọn giữ tiếng gốc")
	}

	args := []string{"-y", "-i", videoPath, "-i", dubWav}
	if keep {
		fc := fmt.Sprintf(
			"[0:a]volume=%.3f[og];[1:a]apad[dub];[og][dub]amix=inputs=2:duration=first:dropout_transition=0:normalize=0[aout]",
			originalVolume(cfg))
		args = append(args, "-filter_complex", fc, "-map", "0:v", "-map", "[aout]")
	} else {
		args = append(args, "-filter_complex", "[1:a]apad[aout]",
			"-map", "0:v", "-map", "[aout]", "-shortest")
	}
	args = append(args, "-c:v", "copy", "-c:a", "aac", "-b:a", "192k", "-movflags", "+faststart", dst)

	if err := runFFmpeg(ctx, args...); err != nil {
		return fmt.Errorf("ghép tiếng vào video thất bại: %w", err)
	}
	return nil
}
