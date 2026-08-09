package vtemplate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bizstudio/internal/media"
)

// ---------- Nhạc nền theo tone ----------
//
// Nhạc nền được TỔNG HỢP tại chỗ bằng ffmpeg, không mang theo thư viện nhạc của
// bên thứ ba — cùng cách đã làm với tiếng động, và vì lý do quan trọng hơn:
// nhạc có bản quyền lọt vào video là bị nền tảng gỡ tiếng hoặc chặn kiếm tiền.
//
// Đây là nhạc NỀN đúng nghĩa: bè đệm giữ không khí, không có giai điệu chính để
// không giành chỗ với lời đọc. Ai muốn nhạc thật thì tự đưa file vào như cũ.

// Mood — một tone nhạc.
type Mood struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Desc  string `json:"desc"`
	lavfi string // chuỗi nguồn + lọc để tổng hợp một vòng lặp
}

const (
	moodRate = 48000
	moodLoop = 20 // giây mỗi vòng; render sẽ lặp lại cho đủ độ dài video
)

var moods = []Mood{
	{
		"hao-hung", "Hào hứng", "Nhịp nảy, sáng — hợp quảng cáo, khuyến mãi, mở hộp",
		"sine=f=220:r=48000:d=20,tremolo=f=4:d=0.6,highpass=f=150,lowpass=f=3000",
	},
	{
		"vui-tuoi", "Vui tươi", "Nhẹ và tươi — hợp hài, mẹo vặt, đời thường",
		"sine=f=330:r=48000:d=20,tremolo=f=6:d=0.5,highpass=f=200,lowpass=f=3500",
	},
	{
		"nhe-nhang", "Nhẹ nhàng", "Bè đệm êm, không giành chỗ với lời — hợp review, hướng dẫn, vlog",
		"sine=f=196:r=48000:d=20,vibrato=f=0.5:d=0.3,lowpass=f=1800",
	},
	{
		"cang-thang", "Căng thẳng", "Dồn nén, giữ người xem — hợp số liệu, so sánh, tóm tắt phim",
		"sine=f=110:r=48000:d=20,tremolo=f=2:d=0.7,lowpass=f=1200",
	},
	{
		"hung-trang", "Hùng tráng", "Trầm và rộng — hợp lịch sử, hồ sơ doanh nghiệp",
		"sine=f=82:r=48000:d=20,vibrato=f=0.3:d=0.4,lowpass=f=900",
	},
	{
		"u-am", "U ám", "Tối, lạnh — hợp kinh dị, chuyện bí ẩn",
		"anoisesrc=r=48000:c=brown:a=0.5:d=20,lowpass=f=400,tremolo=f=0.7:d=0.5",
	},
	{
		"huyen-ao", "Huyền ảo", "Lơ lửng, mơ hồ — hợp thế giới vi mô, chuyện cổ",
		"sine=f=294:r=48000:d=20,vibrato=f=1.2:d=0.6,highpass=f=250,lowpass=f=2200",
	},
}

// moodPeakDb — mọi tone cân về cùng mức đỉnh này. Thấp hơn tiếng động vì nhạc
// chạy suốt video dưới lời đọc, to lên là át lời.
const moodPeakDb = -24.0

// Moods trả bảng tone (bản sao).
func Moods() []Mood {
	out := make([]Mood, len(moods))
	copy(out, moods)
	return out
}

// FindMood tìm tone theo id.
func FindMood(id string) (Mood, bool) {
	for _, m := range moods {
		if strings.EqualFold(m.ID, id) {
			return m, true
		}
	}
	return Mood{}, false
}

// EnsureMood bảo đảm file nhạc của tone đã có trong musicDir, tổng hợp nếu chưa.
func EnsureMood(ctx context.Context, musicDir, id string) (string, error) {
	m, ok := FindMood(id)
	if !ok {
		return "", fmt.Errorf("không có tone nhạc %q", id)
	}
	dst := filepath.Join(musicDir, m.ID+".wav")
	if st, err := os.Stat(dst); err == nil && st.Size() > 0 {
		return dst, nil
	}
	if err := os.MkdirAll(musicDir, 0o755); err != nil {
		return "", fmt.Errorf("tạo thư mục nhạc: %w", err)
	}
	raw := dst + ".raw.wav"
	defer os.Remove(raw)

	// Vào và ra đều mờ dần để chỗ nối vòng lặp không nghe thấy cú "cụp".
	fc := fmt.Sprintf("%s,afade=t=in:st=0:d=2,afade=t=out:st=%d:d=2", m.lavfi, moodLoop-2)
	if err := media.RunFFmpeg(ctx, "-y", "-hide_banner", "-f", "lavfi", "-i", fc,
		"-ac", "1", "-ar", fmt.Sprint(moodRate), "-c:a", "pcm_s16le", raw); err != nil {
		return "", fmt.Errorf("tổng hợp tone nhạc %q thất bại: %w", m.Name, err)
	}
	gain, err := media.PeakGainDb(ctx, raw, moodPeakDb)
	if err != nil {
		return "", err
	}
	if err := media.RunFFmpeg(ctx, "-y", "-hide_banner", "-i", raw,
		"-af", fmt.Sprintf("volume=%.2fdB", gain),
		"-ac", "1", "-ar", fmt.Sprint(moodRate), "-c:a", "pcm_s16le", dst); err != nil {
		return "", fmt.Errorf("cân độ to tone nhạc %q thất bại: %w", m.Name, err)
	}
	return dst, nil
}

// EnsureAllMoods tổng hợp toàn bộ tone (gọi khi mở trang nhạc).
func EnsureAllMoods(ctx context.Context, musicDir string) error {
	for _, m := range moods {
		if _, err := EnsureMood(ctx, musicDir, m.ID); err != nil {
			return err
		}
	}
	return nil
}

// MoodForScript đoán tone hợp với nội dung kịch bản bằng từ khoá.
//
// Cố ý dùng từ khoá chứ không gọi AI: đây là gợi ý mặc định, sai thì người dùng
// đổi một cú bấm — không đáng tốn một lượt gọi AI và vài giây chờ.
// Trả rỗng khi không đủ tin để đoán.
func MoodForScript(text string) string {
	s := strings.ToLower(text)
	hits := map[string]int{}
	for mood, keys := range moodKeywords {
		for _, k := range keys {
			if strings.Contains(s, k) {
				hits[mood]++
			}
		}
	}
	best, bestN := "", 0
	for mood, n := range hits {
		if n > bestN {
			best, bestN = mood, n
		}
	}
	return best
}

var moodKeywords = map[string][]string{
	"u-am":       {"kinh dị", "ma", "rùng rợn", "bí ẩn", "đáng sợ", "ám ảnh", "nghĩa địa", "bóng tối"},
	"hung-trang": {"lịch sử", "chiến", "vua", "triều đại", "anh hùng", "tổ quốc", "khởi nghĩa", "doanh nghiệp", "sứ mệnh"},
	"cang-thang": {"số liệu", "cảnh báo", "nguy cơ", "so sánh", "thống kê", "khủng hoảng", "tăng vọt", "sụt giảm"},
	"vui-tuoi":   {"hài", "buồn cười", "mẹo", "vui", "cười", "lầy", "trò"},
	"hao-hung":   {"khuyến mãi", "giảm giá", "sản phẩm", "mua ngay", "deal", "mở hộp", "ra mắt", "tuyển dụng"},
	"huyen-ao":   {"cổ tích", "huyền", "vi mô", "kỳ lạ", "thần thoại", "vũ trụ", "mơ"},
	"nhe-nhang":  {"hướng dẫn", "review", "vlog", "du lịch", "chia sẻ", "sức khoẻ", "kiến thức"},
}
