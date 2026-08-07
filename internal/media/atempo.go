package media

import (
	"fmt"
	"strings"
)

// AtempoChain sinh chuỗi lọc tăng/giảm tốc audio. Mỗi tầng atempo của ffmpeg
// chỉ nhận 0.5–2.0 nên tốc độ ngoài dải phải xếp nhiều tầng nhân với nhau.
// speed <= 0 hoặc == 1 → "anull" (không đổi).
func AtempoChain(speed float64) string {
	if speed <= 0 || speed == 1 {
		return "anull"
	}
	var parts []string
	for speed > 2.0 {
		parts = append(parts, "atempo=2.0")
		speed /= 2.0
	}
	for speed < 0.5 {
		parts = append(parts, "atempo=0.5")
		speed /= 0.5
	}
	parts = append(parts, fmt.Sprintf("atempo=%.4f", speed))
	return strings.Join(parts, ",")
}
