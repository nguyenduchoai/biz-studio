package vtemplate

import (
	"context"
	"fmt"
	"strings"

	"bizstudio/internal/media"
)

// ---------- Preset xuất theo nền tảng ----------
//
// Mỗi nền tảng có một bộ ràng buộc riêng mà người làm nội dung phải thuộc lòng:
// khung hình, trần thời lượng, độ to chuẩn. Sai độ to là bị nền tảng tự nén cho
// bằng, tiếng nghe bẹt; sai khung hình là bị cắt mất chữ.
//
// Độ to lấy theo chuẩn phát của từng nền tảng (LUFS tích hợp). Đây là mức các
// nền tảng chuẩn hoá về — nộp to hơn thì bị hạ xuống kèm méo, nhỏ hơn thì bị
// nâng lên kèm ồn nền.

// Platform — một preset nền tảng.
type Platform struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Aspect   string  `json:"aspect"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	MaxSec   int     `json:"maxSec"`   // 0 = không giới hạn đáng kể
	LUFS     float64 `json:"lufs"`     // độ to tích hợp mục tiêu
	SafeBot  float64 `json:"safeBot"`  // % dưới bị giao diện che
	SafeSide float64 `json:"safeSide"` // % phải bị che
	Note     string  `json:"note"`
}

var platforms = []Platform{
	{
		ID: "tiktok", Name: "TikTok", Aspect: "9:16", Width: 1080, Height: 1920,
		MaxSec: 600, LUFS: -14, SafeBot: 15, SafeSide: 12,
		Note: "Giao diện che 15% dưới và 12% phải — chữ quan trọng phải nằm ngoài.",
	},
	{
		ID: "reels", Name: "Instagram Reels", Aspect: "9:16", Width: 1080, Height: 1920,
		MaxSec: 180, LUFS: -14, SafeBot: 20, SafeSide: 12,
		Note: "Vùng che dưới rộng hơn TikTok (nút và chú thích chiếm nhiều hơn).",
	},
	{
		ID: "shorts", Name: "YouTube Shorts", Aspect: "9:16", Width: 1080, Height: 1920,
		MaxSec: 180, LUFS: -14, SafeBot: 14, SafeSide: 10,
		Note: "Tối đa 3 phút; dài hơn sẽ bị xếp thành video thường.",
	},
	{
		ID: "youtube", Name: "YouTube (ngang)", Aspect: "16:9", Width: 1920, Height: 1080,
		MaxSec: 0, LUFS: -14, SafeBot: 8, SafeSide: 5,
		Note: "Chừa mép dưới cho thanh điều khiển và thẻ gợi ý.",
	},
	{
		ID: "facebook", Name: "Facebook Reels", Aspect: "9:16", Width: 1080, Height: 1920,
		MaxSec: 90, LUFS: -14, SafeBot: 20, SafeSide: 12,
		Note: "Tối đa 90 giây.",
	},
	{
		ID: "vuong", Name: "Vuông 1:1 (feed)", Aspect: "1:1", Width: 1080, Height: 1080,
		MaxSec: 0, LUFS: -14, SafeBot: 8, SafeSide: 8,
		Note: "Hợp bài đăng trên dòng thời gian, xem không cần xoay máy.",
	},
}

// Platforms trả bảng preset (bản sao).
func Platforms() []Platform {
	out := make([]Platform, len(platforms))
	copy(out, platforms)
	return out
}

// FindPlatform tìm preset theo id.
func FindPlatform(id string) (Platform, bool) {
	for _, p := range platforms {
		if strings.EqualFold(p.ID, id) {
			return p, true
		}
	}
	return Platform{}, false
}

// NormalizeReport — kết quả chuẩn hoá một video cho nền tảng.
type NormalizeReport struct {
	Platform  string  `json:"platform"`
	FromW     int     `json:"fromW"`
	FromH     int     `json:"fromH"`
	ToW       int     `json:"toW"`
	ToH       int     `json:"toH"`
	Padded    bool    `json:"padded"`   // có phải thêm viền vì lệch tỉ lệ
	Duration  float64 `json:"duration"` // thời lượng sau chuẩn hoá
	OverLimit bool    `json:"overLimit"`
	Note      string  `json:"note"`
}

// NormalizeForPlatform đưa video về đúng khung hình và độ to của nền tảng.
//
// KHÔNG tự cắt bớt khi video dài quá trần — cắt video của người khác mà không
// hỏi là chuyện không được làm. Chỉ báo trong report để người dùng tự quyết.
func NormalizeForPlatform(ctx context.Context, src, dst, platformID string) (*NormalizeReport, error) {
	p, ok := FindPlatform(platformID)
	if !ok {
		return nil, fmt.Errorf("không có nền tảng %q", platformID)
	}
	info, err := media.Probe(src)
	if err != nil {
		return nil, err
	}
	rep := &NormalizeReport{
		Platform: p.Name, FromW: info.Width, FromH: info.Height,
		ToW: p.Width, ToH: p.Height, Duration: info.Duration,
	}
	if p.MaxSec > 0 && info.Duration > float64(p.MaxSec) {
		rep.OverLimit = true
		rep.Note = fmt.Sprintf("Video dài %.0f giây, vượt trần %d giây của %s — hệ thống KHÔNG tự cắt, bạn tự cắt lại cho vừa.",
			info.Duration, p.MaxSec, p.Name)
	}
	// scale giữ nguyên tỉ lệ rồi pad cho đủ khung: méo hình là hỏng hẳn, thêm
	// viền thì còn xem được.
	rep.Padded = info.Width*p.Height != info.Height*p.Width
	vf := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,"+
		"pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black,setsar=1", p.Width, p.Height, p.Width, p.Height)

	args := []string{"-y", "-hide_banner", "-i", src, "-vf", vf}
	if media.HasAudio(ctx, src) {
		args = append(args, "-af", fmt.Sprintf("loudnorm=I=%.1f:TP=-1.5:LRA=11", p.LUFS))
	}
	args = append(args,
		"-c:v", "libx264", "-crf", "20", "-preset", "medium", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "192k", "-ar", "48000",
		"-movflags", "+faststart", dst)
	if err := media.RunFFmpeg(ctx, args...); err != nil {
		return nil, fmt.Errorf("chuẩn hoá cho %s thất bại: %w", p.Name, err)
	}
	if out, err := media.Probe(dst); err == nil && out.Duration > 0 {
		rep.Duration = out.Duration
	}
	return rep, nil
}

// AspectSize đổi tỉ lệ khung hình thành kích thước pixel chuẩn. Tỉ lệ lạ thì
// trả về dọc 1080×1920 — mặc định an toàn nhất vì phần lớn video ngắn là dọc.
func AspectSize(aspect string) (w, h int) {
	switch strings.TrimSpace(aspect) {
	case "16:9":
		return 1920, 1080
	case "1:1":
		return 1080, 1080
	default: // "9:16" và mọi thứ khác
		return 1080, 1920
	}
}
