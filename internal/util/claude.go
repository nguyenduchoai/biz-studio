package util

import "strings"

// ClaudeFailReason gộp stderr + stdout của Claude CLI thành thông báo lỗi dễ hiểu.
// Claude CLI in nhiều lỗi quan trọng ra STDOUT (vd "Credit balance is too low"),
// nên chỉ đọc stderr sẽ ra thông báo rỗng, người dùng không biết vì sao hỏng.
func ClaudeFailReason(stdout, stderr string) string {
	msg := strings.TrimSpace(stderr)
	if out := strings.TrimSpace(stdout); out != "" {
		if msg == "" {
			msg = out
		} else {
			msg = msg + " | " + out
		}
	}
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "credit balance"):
		return msg + " — tài khoản Claude đã hết hạn mức. Nạp thêm credit hoặc chờ chu kỳ subscription mới, " +
			"hoặc chuyển sang engine Gemini / API Trực Tiếp trong Cấu hình & API."
	case strings.Contains(low, "rate limit") || strings.Contains(low, "429"):
		return msg + " — đang bị giới hạn tần suất, thử lại sau ít phút."
	case strings.Contains(low, "not logged in") || strings.Contains(low, "unauthorized") || strings.Contains(low, "authentication"):
		return msg + " — Claude CLI chưa đăng nhập. Mở terminal chạy `claude` để đăng nhập subscription."
	}
	if msg == "" {
		return "không có thông báo chi tiết — thử chạy `claude -p \"test\"` trong terminal để xem lỗi"
	}
	if r := []rune(msg); len(r) > 500 {
		return string(r[:500]) + "…"
	}
	return msg
}
