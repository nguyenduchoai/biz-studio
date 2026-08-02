package htmlvideo

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strings"
)

//go:embed templates/scene.html
var templateFS embed.FS

var sceneTpl = template.Must(template.ParseFS(templateFS, "templates/scene.html"))

var validTemplates = map[string]bool{
	"hero": true, "bullets": true, "code": true, "chart": true,
	"product": true, "quote": true, "outro": true,
}

// normalizeTemplate chuẩn hoá tên template; không hợp lệ → "hero".
func normalizeTemplate(v string) string {
	t := strings.ToLower(strings.TrimSpace(v))
	if validTemplates[t] {
		return t
	}
	return "hero"
}

// writeSceneHTML sinh file HTML tĩnh-theo-thời-gian của một cảnh: nhúng JSON
// cảnh + theme + kích thước + thời lượng; trang định nghĩa window.seek(t).
func writeSceneHTML(sc Scene, cfg Config, w, h int, durS float64, imgPath, dst string) error {
	payload := map[string]any{
		"template": normalizeTemplate(sc.Template),
		"title":    strings.TrimSpace(sc.Title),
		"subtitle": strings.TrimSpace(sc.Subtitle),
		"bullets":  sc.Bullets,
		"code":     sc.Code,
		"chart":    sc.Chart,
		"accent":   strings.TrimSpace(sc.Accent),
		"theme":    cfg.theme(),
		"w":        w,
		"h":        h,
		"dur":      durS,
		"image":    "",
	}
	if imgPath != "" {
		payload["image"] = fileURL(imgPath)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mã hoá dữ liệu cảnh: %w", err)
	}
	var b strings.Builder
	if err := sceneTpl.Execute(&b, map[string]any{"DataJSON": template.JS(raw)}); err != nil {
		return fmt.Errorf("dựng HTML cảnh: %w", err)
	}
	if err := os.WriteFile(dst, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("ghi file HTML cảnh %s: %w", dst, err)
	}
	return nil
}
