package media

import (
	"context"
	"fmt"
	"strings"

	"bizstudio/internal/util"
)

// Mức và thời gian của hiệu ứng né giọng (ducking).
const (
	duckRatio   = 4.0  // nén 4:1 — đủ để nhạc lùi lại mà không bị "hẫng"
	duckThresh  = 0.03 // ngưỡng kích hoạt theo biên độ giọng
	duckAttack  = 20   // ms — hạ nhạc nhanh khi bắt đầu nói
	duckRelease = 400  // ms — nâng nhạc lại chậm, tránh giật cục giữa các từ
	duckMakeup  = 1.0
)

// MixBgmDucked trộn nhạc nền vào video/audio và TỰ HẠ NHẠC mỗi khi có tiếng nói.
//
// Khác với cách trộn cũ (nhạc giữ nguyên một mức suốt video, lời đọc phải "cạnh
// tranh" với nhạc), ở đây giọng được dùng làm tín hiệu điều khiển: đang nói thì
// nhạc lùi xuống, hết câu nhạc nâng lại. Nhờ vậy nhạc có thể để to hơn mà lời
// vẫn nghe rõ.
//
// vol là âm lượng nhạc lúc KHÔNG có tiếng nói (0..1). srcHasVideo=false khi
// nguồn chỉ là audio (không map stream hình).
func MixBgmDucked(ctx context.Context, src, bgm, dst string, vol float64, srcHasVideo bool) error {
	if vol <= 0 {
		vol = 0.25
	}
	// [1:a] nhạc: hạ về mức nền → sidechaincompress lấy [0:a] (giọng) làm tín
	// hiệu điều khiển → amix với chính giọng. asplit vì giọng dùng ở 2 nhánh.
	fc := fmt.Sprintf(
		"[0:a]asplit=2[voice][key];"+
			"[1:a]volume=%.3f[bgraw];"+
			"[bgraw][key]sidechaincompress=threshold=%.3f:ratio=%.1f:attack=%d:release=%d:makeup=%.1f[bgduck];"+
			"[voice][bgduck]amix=inputs=2:duration=first:dropout_transition=2:normalize=0[aout]",
		vol, duckThresh, duckRatio, duckAttack, duckRelease, duckMakeup)

	args := []string{
		"-y",
		"-i", src,
		"-stream_loop", "-1", "-i", bgm,
		"-filter_complex", fc,
	}
	if srcHasVideo {
		args = append(args, "-map", "0:v", "-map", "[aout]", "-c:v", "copy")
	} else {
		args = append(args, "-map", "[aout]")
	}
	args = append(args, "-c:a", "aac", "-b:a", "192k", dst)

	if _, err := util.Run(ctx, "ffmpeg", args...); err != nil {
		if strings.Contains(err.Error(), "sidechaincompress") {
			return fmt.Errorf("ffmpeg thiếu bộ lọc sidechaincompress — cập nhật ffmpeg mới hơn: %w", err)
		}
		return fmt.Errorf("trộn nhạc nền (né giọng) thất bại: %w", err)
	}
	return nil
}
