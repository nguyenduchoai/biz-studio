// Package htmlvideo — engine render video MP4 từ các cảnh HTML/CSS bằng
// headless Chrome (chromedp), phong cách "video as code": mỗi cảnh là một
// trang HTML tĩnh theo tham số thời gian (window.seek(t)), chụp từng frame
// qua CDP trong MỘT tiến trình Chrome duy nhất rồi ghép bằng ffmpeg.
package htmlvideo

import (
	"fmt"
	"os"
	"strings"

	"bizstudio/internal/store"
)

// ChartItem — một thanh dữ liệu trong cảnh chart.
type ChartItem struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

// Scene — một cảnh HTML trong video.
type Scene struct {
	Template  string      `json:"template"` // hero | bullets | code | chart | product | quote | outro
	Title     string      `json:"title"`
	Subtitle  string      `json:"subtitle"`
	Bullets   []string    `json:"bullets"`
	Code      string      `json:"code"`
	Image     string      `json:"image"` // path file có sẵn HOẶC từ khóa tìm ảnh (template product)
	Chart     []ChartItem `json:"chart"`
	VoiceText string      `json:"voiceText"`
	Duration  float64     `json:"duration"` // giây; 0 = tự tính
	Accent    string      `json:"accent"`   // màu chủ đạo override, vd "#22D3EE"
}

// Config — cấu hình render HTML Video.
type Config struct {
	Aspect    string  `json:"aspect"` // "9:16" | "16:9" | "1:1" (mặc định 9:16)
	Theme     string  `json:"theme"`  // vivid | dark | light (mặc định vivid)
	FPS       int     `json:"fps"`    // mặc định 24
	Narration bool    `json:"narration"`
	Voice     string  `json:"voice"`
	Engine    string  `json:"engine"` // engine TTS: say | gemini
	BgmPath   string  `json:"bgmPath"`
	BgmVolume float64 `json:"bgmVolume"` // 0..1, mặc định 0.25
	BurnSub   bool    `json:"burnSub"`

	// Kit — bộ style điều khiển giao diện video (font, cỡ chữ, màu, logo, tư
	// liệu nền). nil → Render tự lấy bộ đang mặc định của store; vẫn không có
	// bộ nào → giữ nguyên giao diện mặc định như trước khi có Style Kit.
	Kit *store.StyleKit `json:"kit,omitempty"`

	// Tư liệu của bộ style đã nhúng sẵn dạng data URI — chuẩn bị MỘT lần trong
	// Render rồi truyền xuống từng cảnh (Chrome headless không đọc file ngoài).
	logoURI    string
	stockURIs  []string
	stockURI   string // khung hình nền của riêng cảnh đang dựng
	customWarn bool   // đã cảnh báo CustomHTML thiếu window.seek
}

// stockFor chọn khung hình tư liệu nền cho cảnh thứ i (xoay vòng danh sách).
func (c Config) stockFor(i int) string {
	if len(c.stockURIs) == 0 {
		return ""
	}
	if i < 0 {
		i = 0
	}
	return c.stockURIs[i%len(c.stockURIs)]
}

// kitOf trả bộ style hiệu lực của cấu hình (có thể nil).
func (c Config) kitOf() *store.StyleKit { return c.Kit }

// isCustomKit — bộ style dùng HTML tự viết thay cho template dựng sẵn.
func isCustomKit(k *store.StyleKit) bool {
	return k != nil &&
		strings.EqualFold(strings.TrimSpace(k.BaseTemplate), "custom") &&
		strings.TrimSpace(k.CustomHTML) != ""
}

// resolveSize map tỉ lệ khung hình → kích thước pixel.
func resolveSize(aspect string) (int, int, error) {
	switch strings.TrimSpace(aspect) {
	case "", "9:16":
		return 1080, 1920, nil
	case "16:9":
		return 1920, 1080, nil
	case "1:1":
		return 1080, 1080, nil
	default:
		return 0, 0, fmt.Errorf("tỉ lệ khung hình không hợp lệ: %q (hỗ trợ 9:16, 16:9, 1:1)", aspect)
	}
}

// fps trả FPS hiệu lực (mặc định 24, chặn dải 1..60).
func (c Config) fps() int {
	if c.FPS <= 0 {
		return 24
	}
	if c.FPS > 60 {
		return 60
	}
	return c.FPS
}

// theme trả theme hiệu lực (mặc định "vivid").
func (c Config) theme() string {
	switch strings.ToLower(strings.TrimSpace(c.Theme)) {
	case "dark":
		return "dark"
	case "light":
		return "light"
	default:
		return "vivid"
	}
}

// bgmVolume trả âm lượng nhạc nền hiệu lực (mặc định 0.25).
func (c Config) bgmVolume() float64 {
	if c.BgmVolume <= 0 || c.BgmVolume > 1 {
		return 0.25
	}
	return c.BgmVolume
}

// sceneLabel — nhãn hiển thị của cảnh trong log/tiến độ.
func sceneLabel(sc Scene) string {
	if t := strings.TrimSpace(sc.Title); t != "" {
		return t
	}
	return "(" + strings.TrimSpace(sc.Template) + ")"
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}
