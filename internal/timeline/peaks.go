package timeline

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os/exec"
)

// peakRate — tần số lấy mẫu khi dò sóng âm.
//
// 8 kHz đủ để thấy hình dáng câu nói và khoảng lặng; cao hơn chỉ tốn bộ nhớ
// cho một thứ cuối cùng bị nén xuống vài trăm cột trên màn hình.
const peakRate = 8000

// Peaks trả biên độ lớn nhất trong từng ô thời gian, giá trị 0..1.
//
// Vì sao cần: timeline không có sóng âm thì người dùng phải đoán mò chỗ nào có
// tiếng nói, kéo cắt bằng cảm giác rồi nghe lại — mỗi lần sửa một vòng.
//
// Lấy MAX trong ô chứ không lấy trung bình: trung bình làm một tiếng gõ ngắn
// biến mất hẳn khỏi hình, mà đó lại đúng là chỗ người ta cần thấy để cắt.
func Peaks(ctx context.Context, path string, buckets int) ([]float64, error) {
	if buckets <= 0 {
		buckets = 400
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-i", path,
		"-ac", "1", "-ar", fmt.Sprint(peakRate), "-f", "s16le", "-")
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("đọc sóng âm %s thất bại: %w", path, err)
	}
	n := len(raw) / 2
	if n == 0 {
		return nil, fmt.Errorf("file không có dữ liệu âm thanh: %s", path)
	}
	if buckets > n {
		buckets = n
	}

	out := make([]float64, buckets)
	per := float64(n) / float64(buckets)
	for b := 0; b < buckets; b++ {
		lo := int(float64(b) * per)
		hi := int(float64(b+1) * per)
		if hi > n {
			hi = n
		}
		var peak int16
		for i := lo; i < hi; i++ {
			v := int16(binary.LittleEndian.Uint16(raw[i*2:]))
			if a := abs16(v); a > peak {
				peak = a
			}
		}
		out[b] = float64(peak) / math.MaxInt16
	}
	return out, nil
}

// abs16 lấy trị tuyệt đối an toàn: -32768 không có số đối trong int16, đảo dấu
// thẳng sẽ tràn về chính nó (số âm) và làm hỏng phép so sánh đỉnh.
func abs16(v int16) int16 {
	if v == math.MinInt16 {
		return math.MaxInt16
	}
	if v < 0 {
		return -v
	}
	return v
}
