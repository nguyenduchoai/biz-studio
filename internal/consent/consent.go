// Package consent — cổng xác nhận quyền trước khi nhân bản khuôn mặt hoặc giọng
// của một người thật.
//
// Vì sao cần: Biz Studio nhân bản được giọng từ một clip 3–8 giây và làm một
// tấm ảnh nói theo lời đọc. Hai việc đó chỉ hợp lệ khi người trong ảnh/clip
// đồng ý — và chính người bấm nút là người duy nhất biết điều đó. Phần mềm
// không tự kiểm tra được, nên việc nó làm được là BẮT HỎI, và GHI LẠI câu trả
// lời kèm mốc thời gian.
//
// Chặn ở BACKEND chứ không chỉ ở giao diện: ai gọi API bằng curl hay script đều
// phải đi qua cùng một cổng. Một ô tích ở giao diện chỉ chặn được người dùng
// giao diện.
//
// Đây không phải kiểm chứng pháp lý. Nó là một câu hỏi rõ ràng đặt đúng lúc,
// cộng một dòng nhật ký nói rằng câu hỏi đã được đặt và ai đã trả lời gì.
package consent

import (
	"fmt"
	"strings"
	"time"
)

// Kind — loại nhân bản đang xin phép.
type Kind string

const (
	KindVoice Kind = "voice" // nhân bản giọng từ clip mẫu
	KindFace  Kind = "face"  // làm ảnh chân dung nói theo lời đọc
)

// Grant — câu trả lời của người dùng, lưu lại làm bằng chứng.
type Grant struct {
	Kind      Kind      `json:"kind"`
	Rights    bool      `json:"rights"`    // có quyền sử dụng tư liệu này
	Adult     bool      `json:"adult"`     // người trong tư liệu đủ 18 tuổi
	Permitted bool      `json:"permitted"` // người đó đồng ý cho nhân bản
	Subject   string    `json:"subject"`   // ai — để sau này còn tra lại
	At        time.Time `json:"at"`
}

// Check trả lỗi nếu còn thiếu một xác nhận nào.
//
// Trả lỗi cho TỪNG ô còn thiếu chứ không gộp một câu chung: người dùng bấm
// nhầm một ô mà nhận về "chưa xác nhận" thì phải dò lại cả ba.
func Check(g Grant, kind Kind) error {
	what := "giọng nói"
	if kind == KindFace {
		what = "khuôn mặt"
	}
	var missing []string
	if !g.Rights {
		missing = append(missing, "bạn có quyền sử dụng tư liệu "+what+" này")
	}
	if !g.Adult {
		missing = append(missing, "người trong tư liệu đủ 18 tuổi")
	}
	if !g.Permitted {
		missing = append(missing, "người đó đồng ý cho nhân bản "+what)
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("chưa xác nhận: %s. Nhân bản %s của người khác khi chưa được phép "+
		"là việc bạn phải tự chịu trách nhiệm — Biz Studio không kiểm tra hộ được, nên bắt buộc hỏi",
		strings.Join(missing, "; "), what)
}

// Line dựng một dòng nhật ký cho lượt xác nhận. Ghi vào nhật ký chứ không chỉ
// giữ trong bộ nhớ: bằng chứng chỉ có giá trị khi còn đọc lại được sau này.
func Line(g Grant, kind Kind) string {
	subject := strings.TrimSpace(g.Subject)
	if subject == "" {
		subject = "(không ghi tên)"
	}
	what := "giọng"
	if kind == KindFace {
		what = "khuôn mặt"
	}
	return fmt.Sprintf("Xác nhận quyền nhân bản %s — đối tượng: %s · có quyền dùng: ✓ · đủ 18 tuổi: ✓ · được đồng ý: ✓",
		what, subject)
}
