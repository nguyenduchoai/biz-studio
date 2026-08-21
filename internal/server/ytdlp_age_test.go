package server

import (
	"testing"
	"time"
)

// Bản yt-dlp để lâu là nguyên nhân thật sự của lỗi "HTTP Error 403: Forbidden"
// lúc tải, mà thông báo lỗi không hề nhắc tới phiên bản. Phép đọc tuổi bản này
// là thứ duy nhất biến lỗi đó thành một câu người dùng làm được gì đó.
func TestYtdlpAgeDays(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		version string
		want    int
		wantOK  bool
	}{
		{"bản mới ra hôm nay", "2026.08.21", 0, true},
		{"bản 2 ngày trước", "2026.08.19", 2, true},
		{"đúng bản người dùng gặp lỗi 403", "2026.07.04", 48, true},
		{"vắt qua năm", "2025.12.22", 242, true},
		{"nightly có hậu tố vẫn đọc được ngày", "2026.08.19.232305", 2, true},

		// Không đọc được thì phải im, không được đoán bừa thành "bản cũ".
		{"bản tự dựng", "unknown", 0, false},
		{"thiếu thành phần", "2026.08", 0, false},
		{"ngày không có thật", "2026.13.45", 0, false},
		{"rỗng", "", 0, false},
		// Máy lệch giờ / bản nightly của tương lai: âm ngày là vô nghĩa, bỏ qua.
		{"ngày ở tương lai", "2027.01.01", 0, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ytdlpAgeDays(c.version, now)
			if ok != c.wantOK {
				t.Fatalf("ytdlpAgeDays(%q) ok = %v, muốn %v", c.version, ok, c.wantOK)
			}
			if ok && got != c.want {
				t.Errorf("ytdlpAgeDays(%q) = %d ngày, muốn %d", c.version, got, c.want)
			}
		})
	}
}

// Ngưỡng cảnh báo phải nằm trên nhịp phát hành thật của yt-dlp (1–3 tuần), nếu
// không thì bản vừa mới ra cũng bị gắn nhãn "cũ" và cảnh báo mất giá trị.
func TestYtdlpStaleThresholdAboveReleaseCadence(t *testing.T) {
	const maxCadenceDays = 21
	if ytdlpStaleDays <= maxCadenceDays {
		t.Fatalf("ngưỡng %d ngày quá sát nhịp phát hành %d ngày — sẽ báo động giả",
			ytdlpStaleDays, maxCadenceDays)
	}
}
