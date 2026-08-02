package htmlvideo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"bizstudio/internal/gemini"
	"bizstudio/internal/media"
	"bizstudio/internal/stockmedia"
	"bizstudio/internal/store"
	"bizstudio/internal/tts"
)

const (
	minVoicedDur = 3.0 // giây — sàn thời lượng cảnh có lời đọc
	defaultDur   = 5.0 // giây — cảnh không lời đọc, không khai duration
	voicePadSec  = 0.6 // giây — đệm sau khi lời đọc kết thúc
)

// sceneJob — dữ liệu đã chuẩn bị của một cảnh trước khi chụp frame.
type sceneJob struct {
	scene    Scene
	durS     float64
	wavPath  string // "" nếu cảnh không có thuyết minh
	htmlPath string
	frameDir string
}

// Render dựng video hoàn chỉnh từ các cảnh HTML, trả đường dẫn tuyệt đối MP4
// (workDir/htmlvideo-final.mp4). Khi lỗi giữ nguyên tmp/ để debug; chỉ dọn khi
// thành công. upd nhận progress 0..100 + mô tả bước.
func Render(ctx context.Context, st *store.Store, scenes []Scene, cfg Config, workDir string, upd func(float64, string)) (string, error) {
	if len(scenes) == 0 {
		return "", errors.New("không có cảnh nào để render")
	}
	if upd == nil {
		upd = func(float64, string) {}
	}
	w, h, err := resolveSize(cfg.Aspect)
	if err != nil {
		return "", err
	}
	fps := cfg.fps()
	chromeBin, err := FindChrome(st)
	if err != nil {
		return "", err
	}
	tmpDir := filepath.Join(workDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", fmt.Errorf("không tạo được thư mục làm việc %s: %w", tmpDir, err)
	}

	jobs, err := prepareScenes(ctx, st, scenes, cfg, w, h, tmpDir, upd)
	if err != nil {
		return "", err
	}
	if err := captureAll(ctx, chromeBin, jobs, w, h, fps, upd); err != nil {
		return "", err
	}
	clips, err := buildClips(ctx, jobs, fps, tmpDir, upd)
	if err != nil {
		return "", err
	}
	out, err := assembleFinal(ctx, clips, jobs, cfg, workDir, tmpDir, upd)
	if err != nil {
		return "", err
	}

	upd(99, "Dọn dẹp file tạm…")
	_ = os.RemoveAll(tmpDir)
	if abs, aerr := filepath.Abs(out); aerr == nil {
		return abs, nil
	}
	return out, nil
}

// prepareScenes chuẩn bị audio (TTS) + ảnh + file HTML cho từng cảnh (0→18%).
func prepareScenes(ctx context.Context, st *store.Store, scenes []Scene, cfg Config, w, h int, tmpDir string, upd func(float64, string)) ([]*sceneJob, error) {
	n := len(scenes)
	jobs := make([]*sceneJob, 0, n)
	for i, sc := range scenes {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("render bị hủy: %w", err)
		}
		upd(float64(i)/float64(n)*18, fmt.Sprintf("Chuẩn bị cảnh %d/%d: %s", i+1, n, sceneLabel(sc)))
		sceneDir := filepath.Join(tmpDir, fmt.Sprintf("scene-%d", i))
		if err := os.MkdirAll(sceneDir, 0o755); err != nil {
			return nil, fmt.Errorf("không tạo được thư mục cảnh %s: %w", sceneDir, err)
		}
		j, err := prepareScene(ctx, st, sc, cfg, w, h, sceneDir)
		if err != nil {
			return nil, fmt.Errorf("cảnh %d (%s): %w", i+1, sceneLabel(sc), err)
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// prepareScene dựng dữ liệu 1 cảnh: TTS → thời lượng → ảnh (product) → file HTML.
func prepareScene(ctx context.Context, st *store.Store, sc Scene, cfg Config, w, h int, sceneDir string) (*sceneJob, error) {
	j := &sceneJob{scene: sc, frameDir: filepath.Join(sceneDir, "frames")}

	text := strings.TrimSpace(sc.VoiceText)
	if cfg.Narration && text != "" {
		wav := filepath.Join(sceneDir, "voice.wav")
		if err := tts.Speak(ctx, st, text, cfg.Voice, 0, cfg.Engine, wav); err != nil {
			return nil, fmt.Errorf("đọc giọng (TTS) thất bại: %w", err)
		}
		info, err := media.Probe(wav)
		if err != nil {
			return nil, fmt.Errorf("không đọc được thời lượng audio: %w", err)
		}
		j.wavPath = wav
		j.durS = max(info.Duration+voicePadSec, firstPositive(sc.Duration, minVoicedDur))
	} else {
		j.durS = firstPositive(sc.Duration, defaultDur)
	}

	imgPath := prepareImage(ctx, st, sc, w, h, sceneDir)
	j.htmlPath = filepath.Join(sceneDir, "scene.html")
	if err := writeSceneHTML(sc, cfg, w, h, j.durS, imgPath, j.htmlPath); err != nil {
		return nil, err
	}
	return j, nil
}

// prepareImage lấy ảnh cho template "product" theo chuỗi fallback:
// path tồn tại → copy; từ khóa → Pexels (stockmedia) → Gemini sinh ảnh;
// tất cả lỗi → "" (template tự cân đối, không ảnh). Không bao giờ fatal.
func prepareImage(ctx context.Context, st *store.Store, sc Scene, w, h int, sceneDir string) string {
	raw := strings.TrimSpace(sc.Image)
	if raw == "" || strings.ToLower(strings.TrimSpace(sc.Template)) != "product" {
		return ""
	}
	if fileExists(raw) {
		dst := filepath.Join(sceneDir, "image"+strings.ToLower(filepath.Ext(raw)))
		if err := copyFile(raw, dst); err != nil {
			st.AddLog("warn", "htmlvideo", fmt.Sprintf("Không copy được ảnh %s: %v — render không ảnh", raw, err))
			return ""
		}
		return dst
	}
	img := filepath.Join(sceneDir, "image.png")
	if err := stockmedia.SearchImage(ctx, st, raw, w, h, img); err == nil {
		return img
	} else {
		st.AddLog("warn", "htmlvideo", fmt.Sprintf("Tìm ảnh Pexels %q thất bại: %v — thử Gemini", raw, err))
	}
	if ctx.Err() != nil {
		return ""
	}
	prompt := raw + ", ảnh minh họa sản phẩm chất lượng cao, hiện đại, không chữ"
	if err := gemini.NewFromSettings(st).GenerateImage(ctx, prompt, img); err == nil {
		return img
	} else {
		st.AddLog("warn", "htmlvideo", fmt.Sprintf("Gemini sinh ảnh %q thất bại: %v — render không ảnh", raw, err))
	}
	return ""
}

// captureAll chụp frame mọi cảnh trong MỘT phiên Chrome (18→70%).
func captureAll(ctx context.Context, chromeBin string, jobs []*sceneJob, w, h, fps int, upd func(float64, string)) error {
	br, err := newBrowser(ctx, chromeBin, w, h)
	if err != nil {
		return err
	}
	defer br.close()

	total := 0
	for _, j := range jobs {
		total += frameCount(j.durS, fps)
	}
	done := 0
	for i, j := range jobs {
		upd(18+float64(done)/float64(total)*52,
			fmt.Sprintf("Chụp cảnh %d/%d: %s", i+1, len(jobs), sceneLabel(j.scene)))
		err := br.captureScene(j.htmlPath, j.frameDir, j.durS, fps, func() {
			done++
			upd(18+float64(done)/float64(total)*52, "")
		})
		if err != nil {
			return fmt.Errorf("cảnh %d (%s): %w", i+1, sceneLabel(j.scene), err)
		}
	}
	return nil
}

func firstPositive(vals ...float64) float64 {
	for _, v := range vals {
		if v > 0 {
			return v
		}
	}
	return defaultDur
}

// copyFile copy file nguồn → đích (ảnh người dùng cung cấp vào workDir).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
