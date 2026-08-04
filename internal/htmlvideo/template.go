package htmlvideo

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strings"

	"bizstudio/internal/store"
)

//go:embed templates/scene.html
var templateFS embed.FS

var sceneTpl = template.Must(template.ParseFS(templateFS, "templates/scene.html"))

var validTemplates = map[string]bool{
	"hero": true, "bullets": true, "code": true, "chart": true,
	"product": true, "quote": true, "outro": true, "photo": true, "keys": true,
}

// Cỡ chữ mặc định của Style Kit — đổi cỡ chữ được quy về HỆ SỐ so với các mốc
// này, nhờ vậy bộ style để nguyên mặc định sẽ render y hệt bản chưa có Style Kit.
const (
	defSizeTitle = 48
	defSizeBig   = 150
	defSizeBody  = 22
	minSizeRatio = 0.5
	maxSizeRatio = 2.5
)

// normalizeTemplate chuẩn hoá tên template; không hợp lệ → "hero".
func normalizeTemplate(v string) string {
	t := strings.ToLower(strings.TrimSpace(v))
	if validTemplates[t] {
		return t
	}
	return "hero"
}

// normalizeTransition chuẩn hoá kiểu chuyển cảnh; không hợp lệ → "none".
func normalizeTransition(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "fade":
		return "fade"
	case "dip":
		return "dip"
	default:
		return "none"
	}
}

// normalizeMotion chuẩn hoá kiểu chuyển động; không hợp lệ → "basic".
func normalizeMotion(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), "cinematic") {
		return "cinematic"
	}
	return "basic"
}

// normalizeLogoPos chuẩn hoá vị trí logo; không hợp lệ → "left".
func normalizeLogoPos(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "center":
		return "center"
	case "right":
		return "right"
	default:
		return "left"
	}
}

// writeSceneHTML sinh file HTML tĩnh-theo-thời-gian của một cảnh.
func writeSceneHTML(sc Scene, cfg Config, w, h int, durS float64, imgPath, dst string) error {
	img := ""
	if imgPath != "" {
		img = fileURL(imgPath)
	}
	html, err := buildSceneHTML(sc, cfg, w, h, durS, img)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, []byte(html), 0o644); err != nil {
		return fmt.Errorf("ghi file HTML cảnh %s: %w", dst, err)
	}
	return nil
}

// buildSceneHTML dựng nội dung HTML của một cảnh: nhúng JSON cảnh + bộ style
// (màu, font, cỡ chữ, logo, tư liệu nền) + kích thước + thời lượng; trang tự
// định nghĩa window.seek(t) để mọi frame là hàm thuần của thời điểm t.
// Bộ style đặt BaseTemplate=custom thì dùng HTML tự viết thay template dựng sẵn.
func buildSceneHTML(sc Scene, cfg Config, w, h int, durS float64, imgURI string) (string, error) {
	k := cfg.kitOf()
	if isCustomKit(k) {
		return buildCustomHTML(k, sc, imgURI), nil
	}
	payload := map[string]any{
		"template":   normalizeTemplate(sc.Template),
		"title":      strings.TrimSpace(sc.Title),
		"subtitle":   strings.TrimSpace(sc.Subtitle),
		"bullets":    sc.Bullets,
		"code":       sc.Code,
		"chart":      sc.Chart,
		"accent":     strings.TrimSpace(sc.Accent),
		"theme":      cfg.theme(),
		"w":          w,
		"h":          h,
		"dur":        durS,
		"image":      imgURI,
		"kit":        kitPayload(k, cfg.dataDir),
		"logo":       cfg.logoURI,
		"stock":      cfg.stockURI,
		"safeGuides": cfg.SafeGuides,
		"index":      cfg.sceneIndex,
		"transition": normalizeTransition(cfg.Transition),
		"motion":     normalizeMotion(cfg.Motion),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("mã hoá dữ liệu cảnh: %w", err)
	}
	var b strings.Builder
	if err := sceneTpl.Execute(&b, map[string]any{
		"DataJSON": template.JS(raw),
		"FontCSS":  template.CSS(FontFaceCSS(cfg.dataDir)),
	}); err != nil {
		return "", fmt.Errorf("dựng HTML cảnh: %w", err)
	}
	return b.String(), nil
}

// kitPayload rút phần giao diện của bộ style thành dữ liệu cho trang HTML.
// nil → nil (trang giữ nguyên giao diện mặc định như trước khi có Style Kit).
func kitPayload(k *store.StyleKit, dataDir string) map[string]any {
	if k == nil {
		return nil
	}
	accent := strings.TrimSpace(k.Accent)
	return map[string]any{
		"bgDeep":      strings.TrimSpace(k.BgDeep),
		"textMain":    strings.TrimSpace(k.TextMain),
		"accent":      accent,
		"accent2":     secondAccent(k, accent),
		"fontHead":    WithVietFont(strings.TrimSpace(k.FontHead), dataDir),
		"fontBody":    WithVietFont(strings.TrimSpace(k.FontBody), dataDir),
		"rTitle":      sizeRatio(k.SizeTitle, defSizeTitle),
		"rBig":        sizeRatio(k.SizeBig, defSizeBig),
		"rBody":       sizeRatio(k.SizeBody, defSizeBody),
		"palette":     cleanHexList(k.Palette),
		"channelName": strings.TrimSpace(k.ChannelName),
		"logoPos":     normalizeLogoPos(k.LogoPos),
	}
}

// secondAccent — màu thứ hai của các dải gradient: lấy màu kế tiếp trong bảng
// màu nếu có, không thì dùng lại màu nhấn (gradient một tông vẫn đẹp).
func secondAccent(k *store.StyleKit, accent string) string {
	for _, c := range cleanHexList(k.Palette) {
		if !strings.EqualFold(c, accent) {
			return c
		}
	}
	return accent
}

// cleanHexList bỏ mã màu rỗng trong bảng màu.
func cleanHexList(list []string) []string {
	out := make([]string, 0, len(list))
	for _, c := range list {
		if c = strings.TrimSpace(c); c != "" {
			out = append(out, c)
		}
	}
	return out
}

// sizeRatio đổi cỡ chữ của bộ style thành hệ số so với mốc mặc định, chặn dải
// để cỡ chữ quá tay không làm vỡ bố cục cảnh.
func sizeRatio(v, def int) float64 {
	if v <= 0 || def <= 0 {
		return 1
	}
	r := float64(v) / float64(def)
	if r < minSizeRatio {
		return minSizeRatio
	}
	if r > maxSizeRatio {
		return maxSizeRatio
	}
	return r
}

// ---------- template tự viết ----------

// customVarNames — biến được thay trong CustomHTML trước khi render.
const (
	customStageTag = `id="stage"`
	customSeekShim = `<script>if (typeof window.seek !== 'function') { window.seek = function () {}; }</script>`
	customHead     = `<!doctype html><html lang="vi"><head><meta charset="utf-8">` +
		`<style>*{margin:0;padding:0;box-sizing:border-box}html,body{width:100%;height:100%;overflow:hidden}</style></head><body>`
)

// buildCustomHTML dựng trang từ HTML tự viết của bộ style: thay các biến
// {{TITLE}} {{SUBTITLE}} {{CHANNEL_NAME}} {{ACCENT}} {{BG_DEEP}} {{TEXT_MAIN}}
// {{IMAGE}} rồi bảo đảm trang luôn có #stage và window.seek — thiếu seek thì
// cảnh render tĩnh (mọi frame giống nhau) chứ KHÔNG lỗi.
func buildCustomHTML(k *store.StyleKit, sc Scene, imgURI string) string {
	body := strings.NewReplacer(
		"{{TITLE}}", template.HTMLEscapeString(strings.TrimSpace(sc.Title)),
		"{{SUBTITLE}}", template.HTMLEscapeString(strings.TrimSpace(sc.Subtitle)),
		"{{CHANNEL_NAME}}", template.HTMLEscapeString(strings.TrimSpace(k.ChannelName)),
		"{{ACCENT}}", strings.TrimSpace(k.Accent),
		"{{BG_DEEP}}", strings.TrimSpace(k.BgDeep),
		"{{TEXT_MAIN}}", strings.TrimSpace(k.TextMain),
		"{{IMAGE}}", imgURI,
	).Replace(k.CustomHTML)

	var b strings.Builder
	if !strings.Contains(strings.ToLower(body), "<html") {
		b.WriteString(customHead)
	}
	b.WriteString(body)
	// #stage là mốc chờ trang sẵn sàng trước khi chụp frame — trang tự viết
	// không có thì chèn một thẻ rỗng, không ảnh hưởng gì tới hình.
	if !strings.Contains(body, customStageTag) && !strings.Contains(body, `id='stage'`) {
		b.WriteString(`<div id="stage" style="position:fixed;left:0;top:0;width:0;height:0"></div>`)
	}
	b.WriteString(customSeekShim)
	return b.String()
}

// ---------- xem trước ----------

// PreviewHTML dựng HTML xem trước MỘT cảnh bằng ĐÚNG bộ dựng dùng khi render
// video — sửa gì trong Style Kit là thấy nấy, không cần render thử.
// kit nil → lấy bộ đang mặc định của store. tmpDir dùng để trích khung hình tư
// liệu nền; rỗng → xem trước không có tư liệu nền.
func PreviewHTML(ctx context.Context, st *store.Store, kit *store.StyleKit, sc Scene, cfg Config, tmpDir string) (string, error) {
	w, h, err := resolveSize(cfg.Aspect)
	if err != nil {
		return "", err
	}
	cfg.Kit = resolveKit(st, kit)
	cfg.dataDir = st.DataDir // xem trước phải dùng đúng font như lúc render
	cfg.logoURI, cfg.stockURIs = prepareStyleAssets(ctx, st, cfg.Kit, tmpDir)
	cfg.stockURI = cfg.stockFor(0)
	dur := sc.Duration
	if dur <= 0 {
		dur = defaultDur
	}
	return buildSceneHTML(sc, cfg, w, h, dur, "")
}
