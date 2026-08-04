package media

import (
	"context"
	"fmt"
	"strings"
)

// ---------- Chỉnh màu: 14 preset dựng sẵn ----------
//
// Mỗi preset là một chuỗi lọc ffmpeg thuần, không cần file .cube đi kèm — nhờ
// vậy chạy được ngay trên máy trắng, không phải tải LUT về.
//
// Quy ước dựng: eq lo tương phản/độ bão hoà tổng thể, colorbalance đẩy màu
// riêng cho vùng tối / trung / sáng (hậu tố s/m/h), curves nắn độ sáng.

// GradePreset — một kiểu màu.
type GradePreset struct {
	ID     string `json:"id"`
	Name   string `json:"name"`   // tên tiếng Việt hiện trên giao diện
	Desc   string `json:"desc"`   // khi nào nên dùng
	Filter string `json:"filter"` // chuỗi -vf
}

// gradeList — thứ tự cố định, hiện đúng thứ tự này trên giao diện.
var gradeList = []GradePreset{
	{
		"trong-treo", "Trong trẻo", "Ảnh gốc nhưng sạch và rõ hơn — dùng khi không muốn màu can thiệp nhiều",
		"eq=contrast=1.06:saturation=1.06:gamma=1.02,unsharp=5:5:0.35",
	},
	{
		"dien-anh-lanh", "Điện ảnh lạnh", "Bóng ngả xanh ngọc, da vẫn ấm — kiểu phim hành động, phỏng vấn",
		"colorbalance=rs=-0.06:gs=0.02:bs=0.10:rm=0.02:bm=-0.02:rh=0.06:bh=-0.05,eq=contrast=1.12:saturation=1.02",
	},
	{
		"dien-anh-am", "Điện ảnh ấm", "Vùng sáng ngả vàng mật, bóng nâu — kiểu phim tình cảm, kể chuyện",
		"colorbalance=rs=0.04:bs=-0.05:rh=0.08:gh=0.03:bh=-0.08,eq=contrast=1.10:saturation=1.04:gamma=1.03",
	},
	{
		"hoang-hon", "Hoàng hôn", "Nắng chiều vàng cam — cảnh ngoài trời, du lịch, ẩm thực",
		"colorbalance=rm=0.10:gm=0.03:bm=-0.08:rh=0.12:bh=-0.10,eq=contrast=1.05:saturation=1.14:gamma=1.05",
	},
	{
		"xanh-bien", "Xanh biển", "Đẩy xanh lam/ngọc — biển, hồ bơi, công nghệ",
		"colorbalance=rs=-0.10:gs=0.06:bs=0.20:rm=-0.10:gm=0.06:bm=0.16:rh=-0.06:bh=0.10,eq=contrast=1.08:saturation=1.12",
	},
	{
		"phim-nhua", "Phim nhựa", "Đen không sâu tuyệt đối, tương phản mềm, có hạt — cảm giác quay phim nhựa",
		"curves=all='0/0.06 0.25/0.28 0.75/0.78 1/0.96',eq=contrast=0.96:saturation=0.94,noise=alls=6:allf=t+u",
	},
	{
		"den-trang", "Đen trắng", "Đơn sắc dịu — chân dung, tư liệu",
		"hue=s=0,eq=contrast=1.10:gamma=1.04,unsharp=5:5:0.3",
	},
	{
		"den-trang-manh", "Đen trắng gắt", "Đơn sắc tương phản mạnh — ảnh bìa, tiêu đề gây chú ý",
		"hue=s=0,curves=all='0/0 0.28/0.16 0.72/0.88 1/1',eq=contrast=1.22",
	},
	{
		"pastel", "Pastel", "Màu nhạt, sáng, dịu mắt — mỹ phẩm, thời trang, nội dung nhẹ nhàng",
		"curves=all='0/0.10 0.5/0.56 1/0.97',eq=contrast=0.92:saturation=0.82:brightness=0.03",
	},
	{
		"ruc-ro", "Rực rỡ", "Màu nảy, tương phản cao — nội dung mạng xã hội cần bắt mắt ngay",
		"eq=contrast=1.20:saturation=1.35:gamma=0.98,unsharp=5:5:0.6",
	},
	{
		"hoai-co", "Hoài cổ", "Bạc màu, ngả nâu vàng — hồi tưởng, kể chuyện xưa",
		// colorbalance đặt SAU eq: nếu nhuộm trước rồi mới giảm bão hoà thì
		// chính bước giảm bão hoà kéo màu nâu vừa nhuộm về lại trung tính.
		"curves=all='0/0.12 0.5/0.52 1/0.92',eq=saturation=0.62:contrast=0.94,colorbalance=rs=0.10:bs=-0.12:rm=0.10:gm=0.03:bm=-0.14:rh=0.06:bh=-0.10",
	},
	{
		"cyberpunk", "Cyberpunk", "Bóng tím hồng, sáng xanh ngọc — công nghệ, game, đêm đô thị",
		// hai đầu kéo ngược nhau: bóng đẩy đỏ+lam (tím hồng), sáng rút đỏ đẩy
		// lam+lục (xanh ngọc) — đây là thứ tạo ra vẻ "tách tông" của kiểu này.
		"eq=contrast=1.18:saturation=1.28,colorbalance=rs=0.16:bs=0.16:gm=-0.04:rh=-0.14:gh=0.08:bh=0.18",
	},
	{
		"tai-lieu", "Tài liệu", "Màu trung thực, không kịch tính — tin tức, hướng dẫn, giảng dạy",
		"eq=contrast=1.03:saturation=0.98:gamma=1.01",
	},
	{
		"dem-do-thi", "Đêm đô thị", "Đen sâu, ám xanh lạnh, đèn nổi — cảnh quay đêm, phỏng vấn tối",
		"curves=all='0/0 0.3/0.22 0.8/0.86 1/1',colorbalance=bs=0.12:bm=0.06:rh=0.04:bh=0.06,eq=contrast=1.14:saturation=1.06",
	},
}

// GradePresets trả danh sách preset (bản sao, tránh sửa nhầm bảng gốc).
func GradePresets() []GradePreset {
	out := make([]GradePreset, len(gradeList))
	copy(out, gradeList)
	return out
}

// FindGrade tìm preset theo id.
func FindGrade(id string) (GradePreset, bool) {
	for _, p := range gradeList {
		if strings.EqualFold(p.ID, id) {
			return p, true
		}
	}
	return GradePreset{}, false
}

// gradeVF dựng chuỗi filter_complex có pha trộn theo độ mạnh.
//
// strength < 1 thì tách hình làm hai nhánh: một nhánh giữ nguyên, một nhánh
// chỉnh màu, rồi chồng lên nhau theo độ mờ. Cách này áp dụng được cho MỌI
// chuỗi lọc mà không phải chỉnh tay từng tham số của từng preset.
func gradeVF(filter string, strength float64) string {
	if strength >= 0.999 {
		return "[0:v]" + filter + "[vout]"
	}
	if strength <= 0.001 {
		return "[0:v]null[vout]"
	}
	return fmt.Sprintf("[0:v]split=2[base][grade];[grade]%s[g];[base][g]blend=all_mode=normal:all_opacity=%.3f[vout]",
		filter, strength)
}

// ApplyGrade chỉnh màu cả video. strength 0..1 (0 hoặc âm = 1.0 — dùng hết).
func ApplyGrade(ctx context.Context, src, presetID, dst string, strength float64) error {
	p, ok := FindGrade(presetID)
	if !ok {
		return fmt.Errorf("không có kiểu màu %q", presetID)
	}
	if strength <= 0 {
		strength = 1
	}
	if strength > 1 {
		strength = 1
	}
	if err := ensureDir(dst); err != nil {
		return err
	}
	args := []string{"-y", "-hide_banner", "-i", src,
		"-filter_complex", gradeVF(p.Filter, strength),
		"-map", "[vout]"}
	if HasAudio(ctx, src) {
		args = append(args, "-map", "0:a", "-c:a", "copy")
	}
	args = append(args, "-c:v", "libx264", "-crf", "20", "-preset", "medium", dst)
	if err := run(ctx, args...); err != nil {
		return fmt.Errorf("chỉnh màu %q thất bại: %w", p.Name, err)
	}
	return nil
}

// GradePreview xuất MỘT khung hình đã chỉnh màu ra ảnh, để xem thử trước khi
// chạy cả video (rẻ hơn nhiều so với render toàn bộ).
func GradePreview(ctx context.Context, src, presetID, dst string, atSec, strength float64) error {
	p, ok := FindGrade(presetID)
	if !ok {
		return fmt.Errorf("không có kiểu màu %q", presetID)
	}
	if strength <= 0 {
		strength = 1
	}
	if atSec < 0 {
		atSec = 0
	}
	if err := ensureDir(dst); err != nil {
		return err
	}
	if err := run(ctx, "-y", "-hide_banner", "-ss", fmt.Sprintf("%.3f", atSec), "-i", src,
		"-filter_complex", gradeVF(p.Filter, strength),
		"-map", "[vout]", "-frames:v", "1", "-q:v", "2", dst); err != nil {
		return fmt.Errorf("xem thử màu %q thất bại: %w", p.Name, err)
	}
	return nil
}
