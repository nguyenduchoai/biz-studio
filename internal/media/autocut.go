package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"bizstudio/internal/util"
)

const (
	cutPad     = 0.15 // đệm mỗi đầu đoạn giữ (giây)
	cutMinKeep = 0.2  // bỏ đoạn giữ ngắn hơn (giây)
)

// segment — một đoạn video được giữ lại [start, end].
type segment struct {
	start, end float64
}

// AutoCut cắt bỏ các khoảng lặng trong video (jump-cut tự động).
// silenceDb=0 → -35dB, minSil=0 → 0.8s. upd nhận progress 0..100 + mô tả bước.
func AutoCut(ctx context.Context, src, dst string, silenceDb, minSil float64, upd func(float64, string)) error {
	if upd == nil {
		upd = func(float64, string) {}
	}
	if silenceDb == 0 {
		silenceDb = -35
	}
	if minSil == 0 {
		minSil = 0.8
	}
	info, err := Probe(src)
	if err != nil {
		return err
	}
	if info.Duration <= 0 {
		return fmt.Errorf("video không có thời lượng hợp lệ: %s", src)
	}
	if err := ensureDir(dst); err != nil {
		return err
	}
	if !HasAudio(ctx, src) {
		upd(90, "Video không có âm thanh — giữ nguyên bản gốc")
		return copyFile(src, dst)
	}

	upd(5, "Bước 1/3: phân tích khoảng lặng…")
	_, se, runErr := util.RunErr(ctx, "ffmpeg", "-hide_banner", "-i", src,
		"-af", fmt.Sprintf("silencedetect=noise=%.1fdB:d=%.2f", silenceDb, minSil),
		"-f", "null", "-")
	if runErr != nil {
		return fmt.Errorf("phân tích khoảng lặng thất bại: %w — %s", runErr, tail(se, 400))
	}
	silences := ParseSilences(se, info.Duration)
	if len(silences) == 0 {
		upd(90, "Không phát hiện khoảng lặng — giữ nguyên bản gốc")
		return copyFile(src, dst)
	}

	upd(35, "Bước 2/3: tính các đoạn giữ lại…")
	keeps := computeKeeps(silences, info.Duration)
	if len(keeps) == 0 {
		return fmt.Errorf("video gần như im lặng hoàn toàn, không còn đoạn nào để giữ")
	}

	upd(45, fmt.Sprintf("Bước 3/3: cắt và ghép %d đoạn…", len(keeps)))
	if err := cutAndConcat(ctx, src, dst, keeps); err != nil {
		return err
	}
	upd(98, "Đã cắt xong khoảng lặng")
	return nil
}

// computeKeeps tính các đoạn GIỮ (ngoài khoảng lặng), đệm cutPad mỗi đầu,
// bỏ đoạn ngắn hơn cutMinKeep.
func computeKeeps(silences []TimeSpan, dur float64) []segment {
	var keeps []segment
	cur := 0.0
	for _, s := range silences {
		end := s.Start + cutPad
		if end > dur {
			end = dur
		}
		if end-cur >= cutMinKeep {
			keeps = append(keeps, segment{cur, end})
		}
		next := s.End - cutPad
		if next < cur {
			next = cur
		}
		cur = next
	}
	if dur-cur >= cutMinKeep {
		keeps = append(keeps, segment{cur, dur})
	}
	return keeps
}

// cutAndConcat cắt các đoạn giữ bằng trim/atrim + concat, re-encode libx264+aac.
func cutAndConcat(ctx context.Context, src, dst string, keeps []segment) error {
	var b strings.Builder
	for i, k := range keeps {
		fmt.Fprintf(&b, "[0:v]trim=start=%.3f:end=%.3f,setpts=PTS-STARTPTS[v%d];", k.start, k.end, i)
		fmt.Fprintf(&b, "[0:a]atrim=start=%.3f:end=%.3f,asetpts=PTS-STARTPTS[a%d];", k.start, k.end, i)
	}
	for i := range keeps {
		fmt.Fprintf(&b, "[v%d][a%d]", i, i)
	}
	fmt.Fprintf(&b, "concat=n=%d:v=1:a=1[vout][aout]", len(keeps))

	return run(ctx, "-y", "-hide_banner", "-i", src,
		"-filter_complex", b.String(),
		"-map", "[vout]", "-map", "[aout]",
		"-c:v", "libx264", "-crf", "20", "-preset", "veryfast",
		"-c:a", "aac", "-b:a", "192k", dst)
}

// copyFile sao chép nguyên file (khi không có gì để cắt).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("không mở được file nguồn: %w", err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("không tạo được file đích: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("sao chép file thất bại: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("không đóng được file đích: %w", err)
	}
	return nil
}
