package vox

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"bizstudio/internal/gemini"
	"bizstudio/internal/media"
	"bizstudio/internal/stockmedia"
	"bizstudio/internal/store"
	"bizstudio/internal/tts"
	"bizstudio/internal/util"
)

const (
	minSceneDur     = 2.0 // giây — thời lượng tối thiểu mỗi cảnh
	defaultSceneDur = 4.0 // giây — khi không có lời đọc lẫn duration
	titleShowSec    = 3.5 // giây — tiêu đề chỉ hiện ở đầu cảnh
)

var fontCandidates = []string{
	"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
	"/System/Library/Fonts/Helvetica.ttc",
}

// buildScene dựng 1 cảnh: audio (TTS/silence) → ảnh nền → clip mp4.
// Trả (đường dẫn clip, thời lượng thực tế giây, lỗi).
func buildScene(ctx context.Context, st *store.Store, sc Scene, idx int, cfg Config, w, h int, tmpDir string) (string, float64, error) {
	wav, dur, err := buildSceneAudio(ctx, st, sc, idx, cfg, tmpDir)
	if err != nil {
		return "", 0, err
	}
	img, err := buildSceneImage(ctx, st, sc, idx, cfg, w, h, tmpDir)
	if err != nil {
		return "", 0, err
	}
	clip := filepath.Join(tmpDir, fmt.Sprintf("clip-%d.mp4", idx))
	if err := buildSceneClip(ctx, img, wav, clip, dur, sc.Title, cfg, w, h); err != nil {
		return "", 0, err
	}
	return clip, dur, nil
}

// buildSceneAudio tạo file wav cho cảnh: TTS nếu có lời đọc, ngược lại là im lặng
// theo Scene.Duration (mặc định 4s). Thời lượng luôn ≥ 2s.
func buildSceneAudio(ctx context.Context, st *store.Store, sc Scene, idx int, cfg Config, tmpDir string) (string, float64, error) {
	wav := filepath.Join(tmpDir, fmt.Sprintf("scene-%d.wav", idx))
	text := strings.TrimSpace(sc.VoiceText)
	if text == "" {
		d := sc.Duration
		if d <= 0 {
			d = defaultSceneDur
		}
		if d < minSceneDur {
			d = minSceneDur
		}
		if err := makeSilence(ctx, wav, d); err != nil {
			return "", 0, fmt.Errorf("tạo audio im lặng thất bại: %w", err)
		}
		return wav, d, nil
	}
	if err := tts.Speak(ctx, st, text, cfg.Voice, 0, cfg.Engine, wav); err != nil {
		return "", 0, fmt.Errorf("đọc giọng (TTS) thất bại: %w", err)
	}
	info, err := media.Probe(wav)
	if err != nil {
		return "", 0, fmt.Errorf("không đọc được thời lượng audio: %w", err)
	}
	d := info.Duration
	if d < minSceneDur {
		d = minSceneDur
	}
	return wav, d, nil
}

func makeSilence(ctx context.Context, dst string, dur float64) error {
	_, err := util.Run(ctx, "ffmpeg", "-y",
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo",
		"-t", fmt.Sprintf("%.3f", dur),
		"-c:a", "pcm_s16le", dst)
	return err
}

// buildSceneImage chọn ảnh nền theo thứ tự ưu tiên:
// 1) MediaPath người dùng cung cấp (video → lấy frame giữa),
// 2) ảnh stock Pexels theo MediaKeyword (nếu có Pexels API key),
// 3) Gemini sinh ảnh (nếu có API key),
// 4) card màu nền tối.
func buildSceneImage(ctx context.Context, st *store.Store, sc Scene, idx int, cfg Config, w, h int, tmpDir string) (string, error) {
	img := filepath.Join(tmpDir, fmt.Sprintf("img-%d.png", idx))

	if sc.MediaPath != "" && fileExists(sc.MediaPath) {
		if !isVideo(sc.MediaPath) {
			return sc.MediaPath, nil
		}
		t := 0.0
		if info, err := media.Probe(sc.MediaPath); err == nil && info.Duration > 0 {
			t = info.Duration / 2
		}
		if err := media.Thumbnail(sc.MediaPath, img, t, w); err == nil {
			return img, nil
		} else {
			st.AddLog("warn", "vox", fmt.Sprintf("Cảnh %d: không lấy được khung hình từ %s: %v — dùng phương án dự phòng", idx+1, sc.MediaPath, err))
		}
	}

	if kw := strings.TrimSpace(sc.MediaKeyword); kw != "" && strings.TrimSpace(st.Settings().PexelsKey) != "" {
		if err := stockmedia.SearchImage(ctx, st, kw, w, h, img); err == nil {
			return img, nil
		} else {
			if ctx.Err() != nil {
				return "", fmt.Errorf("render bị hủy: %w", ctx.Err())
			}
			st.AddLog("warn", "vox", fmt.Sprintf("Cảnh %d: tìm ảnh stock Pexels cho %q thất bại: %v — dùng phương án dự phòng", idx+1, kw, err))
		}
	}

	if strings.TrimSpace(st.Settings().GeminiAPIKey) != "" {
		if err := gemini.NewFromSettings(st).GenerateImage(ctx, imagePrompt(sc, cfg.Theme), img); err == nil {
			return img, nil
		} else {
			if ctx.Err() != nil {
				return "", fmt.Errorf("render bị hủy: %w", ctx.Err())
			}
			st.AddLog("warn", "vox", fmt.Sprintf("Cảnh %d: Gemini sinh ảnh thất bại: %v — dùng card màu", idx+1, err))
		}
	}

	if err := colorCard(ctx, img, w, h); err != nil {
		return "", fmt.Errorf("tạo ảnh nền thất bại: %w", err)
	}
	return img, nil
}

func imagePrompt(sc Scene, theme string) string {
	var parts []string
	if t := strings.TrimSpace(sc.Title); t != "" {
		parts = append(parts, t)
	}
	if k := strings.TrimSpace(sc.MediaKeyword); k != "" {
		parts = append(parts, k)
	}
	if th := strings.TrimSpace(theme); th != "" {
		parts = append(parts, "chủ đề: "+th)
	}
	parts = append(parts, "ảnh minh họa chất lượng cao, phong cách nhất quán, không chữ")
	return strings.Join(parts, ", ")
}

func colorCard(ctx context.Context, dst string, w, h int) error {
	_, err := util.Run(ctx, "ffmpeg", "-y",
		"-f", "lavfi", "-i", fmt.Sprintf("color=c=0x0F172A:s=%dx%d", w, h),
		"-frames:v", "1", dst)
	return err
}

// buildSceneClip ghép ảnh + audio thành clip mp4: scale/pad về WxH, vẽ tiêu đề
// 3.5s đầu. Không dùng zoompan để ưu tiên ổn định trên mọi bản ffmpeg.
func buildSceneClip(ctx context.Context, img, wav, dst string, dur float64, title string, cfg Config, w, h int) error {
	vf := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=0x0F172A", w, h, w, h)
	if dt := drawTitleFilter(title, w); dt != "" {
		vf += "," + dt
	}
	args := []string{
		"-y",
		"-loop", "1", "-i", img,
		"-i", wav,
		"-t", fmt.Sprintf("%.3f", dur),
		"-vf", vf,
		"-af", "apad", // đệm im lặng nếu audio ngắn hơn -t
		"-r", "30",
		"-c:v", "libx264", "-pix_fmt", "yuv420p",
	}
	args = append(args, qualityArgs(cfg.Quality)...)
	args = append(args, "-c:a", "aac", "-b:a", "192k", dst)
	if _, err := util.Run(ctx, "ffmpeg", args...); err != nil {
		return fmt.Errorf("render clip thất bại: %w", err)
	}
	return nil
}

// drawTitleFilter tạo filter drawtext cho tiêu đề (giữa - dưới, hiện 3.5s đầu).
// Trả "" nếu không có tiêu đề hoặc không tìm thấy font.
func drawTitleFilter(title string, w int) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	font := findFont()
	if font == "" {
		return ""
	}
	return fmt.Sprintf(
		"drawtext=fontfile=%s:text=%s:fontsize=%d:fontcolor=white:box=1:boxcolor=0x2563EB@0.75:boxborderw=18:x=(w-text_w)/2:y=h*0.82:enable='lt(t,%.1f)'",
		escapeDrawText(font), escapeDrawText(title), w/22, titleShowSec,
	)
}

func findFont() string {
	for _, f := range fontCandidates {
		if fileExists(f) {
			return f
		}
	}
	return ""
}

// escapeDrawText escape chuỗi cho drawtext qua 2 tầng parse của ffmpeg
// (filtergraph parser rồi drawtext expansion): \ ' : , ; [ ] %.
func escapeDrawText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\\\`)
		case '\'':
			b.WriteString(`\'`)
		case ':':
			b.WriteString(`\:`)
		case ',':
			b.WriteString(`\,`)
		case ';':
			b.WriteString(`\;`)
		case '[':
			b.WriteString(`\[`)
		case ']':
			b.WriteString(`\]`)
		case '%':
			b.WriteString(`\\%`)
		case '\n', '\r', '\t':
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// qualityArgs map Config.Quality → tham số encode x264.
func qualityArgs(q string) []string {
	switch strings.ToLower(strings.TrimSpace(q)) {
	case "high", "cao":
		return []string{"-crf", "18", "-preset", "medium"}
	case "low", "fast", "nhanh", "thấp":
		return []string{"-crf", "28", "-preset", "veryfast"}
	default:
		return []string{"-crf", "23", "-preset", "veryfast"}
	}
}

func isVideo(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".mov", ".mkv", ".webm", ".avi", ".m4v", ".ts", ".flv", ".wmv", ".mpg", ".mpeg":
		return true
	}
	return false
}
