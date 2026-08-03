package dubbing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// minGap — khoảng trống ngắn hơn mức này thì bỏ qua, không tạo file im lặng riêng.
const minGap = 0.02

// buildTrack ghép các đoạn đã lồng tiếng thành một track duy nhất theo đúng mốc
// thời gian: chèn im lặng cho khoảng trống trước mỗi câu, câu chồng lấn (câu
// trước đọc lấn sang) thì nối liền — không tạo khoảng lặng âm.
//
// Dùng concat tuần tự thay cho amix nhiều input vì SRT có thể có hàng trăm cue.
func buildTrack(ctx context.Context, segs []segment, tmpDir, dst string, upd func(float64, string)) error {
	if len(segs) == 0 {
		return fmt.Errorf("không có câu nào để ghép thành track lồng tiếng")
	}
	upd(progTrack, fmt.Sprintf("Ghép track lồng tiếng (%d câu)…", len(segs)))

	parts := make([]string, 0, len(segs)*2)
	cursor := 0.0
	for i, s := range segs {
		if gap := s.Start - cursor; gap > minGap {
			sil := filepath.Join(tmpDir, fmt.Sprintf("sil-%04d.wav", i+1))
			if err := makeSilence(ctx, gap, sil); err != nil {
				return fmt.Errorf("tạo khoảng lặng trước câu %d: %w", i+1, err)
			}
			parts = append(parts, sil)
			cursor += gap
		}
		parts = append(parts, s.Path)
		cursor += s.Dur
	}
	return concatAudio(ctx, parts, tmpDir, dst)
}

// makeSilence sinh file im lặng dài sec giây, cùng chuẩn với các đoạn khác.
func makeSilence(ctx context.Context, sec float64, dst string) error {
	if sec <= 0 {
		return fmt.Errorf("độ dài khoảng lặng không hợp lệ: %.3fs", sec)
	}
	return runFFmpeg(ctx, "-y",
		"-f", "lavfi", "-i", fmt.Sprintf("anullsrc=r=%d:cl=mono", segRate),
		"-t", fmt.Sprintf("%.3f", sec), "-c:a", "pcm_s16le", dst)
}

// concatAudio ghép tuần tự các file wav cùng chuẩn bằng concat demuxer.
func concatAudio(ctx context.Context, parts []string, tmpDir, dst string) error {
	if len(parts) == 0 {
		return fmt.Errorf("không có đoạn audio nào để ghép")
	}
	list, err := writeConcatList(parts, tmpDir)
	if err != nil {
		return err
	}
	if err := runFFmpeg(ctx, "-y", "-f", "concat", "-safe", "0", "-i", list,
		"-ar", strconv.Itoa(segRate), "-ac", "1", "-c:a", "pcm_s16le", dst); err != nil {
		return fmt.Errorf("ghép track lồng tiếng thất bại: %w", err)
	}
	return nil
}

// writeConcatList ghi file danh sách cho concat demuxer (đường dẫn tuyệt đối,
// escape dấu nháy đơn).
func writeConcatList(parts []string, dir string) (string, error) {
	var b strings.Builder
	for _, p := range parts {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		fmt.Fprintf(&b, "file '%s'\n", strings.ReplaceAll(abs, "'", `'\''`))
	}
	list := filepath.Join(dir, "concat-dub.txt")
	if err := os.WriteFile(list, []byte(b.String()), 0o644); err != nil {
		return "", fmt.Errorf("không ghi được danh sách ghép %s: %w", list, err)
	}
	return list, nil
}
