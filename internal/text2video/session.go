package text2video

import (
	"strings"
	"unicode/utf8"

	"bizstudio/internal/store"
)

// NormalizeSegments chuẩn hoá kịch bản người dùng gửi lên: bỏ đoạn rỗng, tính
// lại số ký tự và xoá thời lượng đo được của đoạn đã bị sửa nội dung (thời
// lượng cũ không còn đúng với lời đọc mới).
// changed = true khi kịch bản khác bản đang lưu.
func NormalizeSegments(old, in []store.T2VSegment) ([]store.T2VSegment, bool) {
	out := make([]store.T2VSegment, 0, len(in))
	changed := len(old) != len(in)
	for i, s := range in {
		s.Text = strings.TrimSpace(s.Text)
		if s.Text == "" {
			changed = true
			continue
		}
		s.Chars = utf8.RuneCountInString(s.Text)
		if i >= len(old) || old[i].Text != s.Text {
			changed = true
			s.Seconds, s.AudioPath = 0, ""
		}
		out = append(out, s)
	}
	return out, changed
}

// InvalidateVoice bỏ giọng đọc cũ khi kịch bản đổi — buộc đọc lại trước khi
// dựng video, tránh hình lệch khỏi giọng.
func InvalidateVoice(sess *store.T2VSession) {
	if sess == nil || sess.VoicePath == "" {
		return
	}
	sess.VoicePath, sess.TranscriptPath, sess.VoiceSeconds = "", "", 0
	if sess.Status == "voice" || sess.Status == "done" {
		sess.Status = "script"
	}
	if sess.Step > 2 {
		sess.Step = 2
	}
}
