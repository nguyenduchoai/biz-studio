package htmlvideo

import (
	"context"
	"encoding/base64"
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
	"bizstudio/internal/stylekit"
	"bizstudio/internal/tts"
)

const (
	minVoicedDur = 3.0 // giây — sàn thời lượng cảnh có lời đọc
	defaultDur   = 5.0 // giây — cảnh không lời đọc, không khai duration
	voicePadSec  = 0.6 // giây — đệm sau khi lời đọc kết thúc

	maxStockBg    = 6             // số tư liệu nền tối đa dùng luân phiên cho một video
	maxEmbedBytes = 8 << 20       // trần dung lượng file nhúng base64 vào HTML
	stockBgWidth  = 1280          // bề rộng khung hình nền sau khi trích (đủ nét, nhẹ)
	logoNoteTag   = "htmlvideo"   // nhãn nguồn log của engine render
	seekFuncMark  = "window.seek" // dấu hiệu CustomHTML có tự điều khiển thời gian
)

// imageTemplates — các template có dùng ảnh trong cảnh.
var imageTemplates = map[string]bool{"product": true, "photo": true}

// imageHints — mô tả thêm khi phải nhờ Gemini sinh ảnh cho từng loại template.
var imageHints = map[string]string{
	"product": "ảnh minh họa sản phẩm chất lượng cao, hiện đại, không chữ",
	"photo":   "ảnh cảnh thật như trong phim, bố cục thoáng, ánh sáng đẹp, không chữ",
}

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
	// Font tiếng Việt nhúng sẵn: có thì mọi máy render ra cùng một kiểu chữ và
	// không lo thiếu dấu; chưa tải thì để trống, trang dùng font hệ điều hành.
	cfg.dataDir = st.DataDir
	tmpDir := filepath.Join(workDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", fmt.Errorf("không tạo được thư mục làm việc %s: %w", tmpDir, err)
	}

	// Bộ style điều khiển toàn bộ giao diện video — nạp MỘT lần rồi truyền
	// xuống mọi cảnh (logo/tư liệu nền nhúng base64 để Chrome chạy offline).
	cfg.Kit = resolveKit(st, cfg.Kit)
	cfg.logoURI, cfg.stockURIs = prepareStyleAssets(ctx, st, cfg.Kit, tmpDir)
	cfg.customWarn = warnCustomTemplate(st, cfg.Kit)

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
		j, err := prepareScene(ctx, st, sc, cfg, w, h, sceneDir, i)
		if err != nil {
			return nil, fmt.Errorf("cảnh %d (%s): %w", i+1, sceneLabel(sc), err)
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// prepareScene dựng dữ liệu 1 cảnh: TTS → thời lượng → ảnh (product) → file HTML.
// idx là số thứ tự cảnh, dùng để luân phiên tư liệu nền của bộ style.
func prepareScene(ctx context.Context, st *store.Store, sc Scene, cfg Config, w, h int, sceneDir string, idx int) (*sceneJob, error) {
	j := &sceneJob{scene: sc, frameDir: filepath.Join(sceneDir, "frames")}
	cfg.stockURI = cfg.stockFor(idx)

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

// prepareImage lấy ảnh cho template có hình ("product", "photo") theo chuỗi
// fallback: path tồn tại → copy; từ khóa → Pexels (stockmedia) → Gemini sinh ảnh;
// tất cả lỗi → "" (template tự cân đối, không ảnh). Không bao giờ fatal.
func prepareImage(ctx context.Context, st *store.Store, sc Scene, w, h int, sceneDir string) string {
	raw := strings.TrimSpace(sc.Image)
	tpl := strings.ToLower(strings.TrimSpace(sc.Template))
	if raw == "" || !imageTemplates[tpl] {
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
	prompt := stylekit.Apply(st, raw+", "+imageHints[tpl])
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

// ---------- tư liệu của bộ style ----------

// resolveKit chọn bộ style hiệu lực: chỉ định sẵn → dùng luôn; không có → bộ
// đang mặc định của store; store cũng không có → nil (giữ nguyên giao diện cũ).
func resolveKit(st *store.Store, k *store.StyleKit) *store.StyleKit {
	if k != nil {
		return k
	}
	if st == nil {
		return nil
	}
	if active, ok := st.ActiveStyleKit(); ok {
		return &active
	}
	return nil
}

// prepareStyleAssets nhúng logo + tư liệu nền của bộ style thành data URI để
// trang HTML chạy được hoàn toàn offline trong Chrome headless. Tư liệu nền là
// video thì trích MỘT khung hình đại diện bằng ffmpeg rồi dùng như ảnh — nhờ
// vậy nền vẫn tất định theo seek(t), không tự chạy lệch giữa các frame.
// Mọi lỗi chỉ ghi log rồi bỏ qua phần đó, KHÔNG bao giờ làm hỏng render.
func prepareStyleAssets(ctx context.Context, st *store.Store, k *store.StyleKit, tmpDir string) (string, []string) {
	if k == nil || st == nil {
		return "", nil
	}
	logoURI := ""
	if p := styleAssetPath(st, k.LogoPath); p != "" {
		uri, err := imageDataURI(p)
		if err != nil {
			st.AddLog("warn", logoNoteTag, fmt.Sprintf("Không nhúng được logo %s: %v — render không logo", k.LogoPath, err))
		} else {
			logoURI = uri
		}
	}
	return logoURI, prepareStockURIs(ctx, st, k, tmpDir)
}

// prepareStockURIs trích khung hình đại diện của tối đa maxStockBg tư liệu nền.
func prepareStockURIs(ctx context.Context, st *store.Store, k *store.StyleKit, tmpDir string) []string {
	if tmpDir == "" || len(k.StockPaths) == 0 {
		return nil
	}
	out := make([]string, 0, maxStockBg)
	for _, rel := range k.StockPaths {
		if len(out) >= maxStockBg || ctx.Err() != nil {
			break
		}
		src := styleAssetPath(st, rel)
		if src == "" {
			continue
		}
		dst := filepath.Join(tmpDir, fmt.Sprintf("stockbg-%d.jpg", len(out)))
		if err := media.Thumbnail(src, dst, 0, stockBgWidth); err != nil {
			st.AddLog("warn", logoNoteTag, fmt.Sprintf("Không đọc được tư liệu nền %s: %v — bỏ qua", rel, err))
			continue
		}
		uri, err := imageDataURI(dst)
		if err != nil {
			st.AddLog("warn", logoNoteTag, fmt.Sprintf("Không nhúng được tư liệu nền %s: %v — bỏ qua", rel, err))
			continue
		}
		out = append(out, uri)
	}
	return out
}

// warnCustomTemplate cảnh báo khi HTML tự viết không định nghĩa window.seek(t):
// vẫn render được nhưng mọi frame giống nhau (cảnh tĩnh). Trả true nếu thiếu.
func warnCustomTemplate(st *store.Store, k *store.StyleKit) bool {
	if !isCustomKit(k) || strings.Contains(k.CustomHTML, seekFuncMark) {
		return false
	}
	if st != nil {
		st.AddLog("warn", logoNoteTag,
			fmt.Sprintf("Template tuỳ chỉnh của bộ style %q chưa định nghĩa window.seek(t) — cảnh sẽ render tĩnh (mọi frame giống nhau)", k.Name))
	}
	return true
}

// styleAssetPath đổi đường dẫn tương đối DataDir → tuyệt đối; rỗng/không tồn
// tại → "" (người dùng có thể đã xoá file bên ngoài app).
func styleAssetPath(st *store.Store, rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" || st == nil {
		return ""
	}
	p := rel
	if !filepath.IsAbs(p) {
		p = filepath.Join(st.DataDir, filepath.FromSlash(rel))
	}
	if !fileExists(p) {
		return ""
	}
	return p
}

// imageDataURI đọc file ảnh → chuỗi data:<mime>;base64,… nhúng thẳng vào HTML.
func imageDataURI(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if fi.Size() == 0 {
		return "", errors.New("file rỗng")
	}
	if fi.Size() > maxEmbedBytes {
		return "", fmt.Errorf("file nặng %.1f MB — vượt trần %d MB khi nhúng vào HTML",
			float64(fi.Size())/(1<<20), maxEmbedBytes>>20)
	}
	mime := imageMIME(filepath.Ext(path))
	if mime == "" {
		return "", fmt.Errorf("định dạng ảnh không hỗ trợ: %s", filepath.Ext(path))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw), nil
}

// imageMIME map đuôi file → kiểu MIME ảnh mà Chrome hiển thị được.
func imageMIME(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	default:
		return ""
	}
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
