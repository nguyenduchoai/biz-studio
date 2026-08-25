package server

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bizstudio/internal/gemini"
	"bizstudio/internal/htmlvideo"
	"bizstudio/internal/media"
	"bizstudio/internal/store"
	"bizstudio/internal/stylekit"
	"bizstudio/internal/util"
)

const (
	// Chủ thể mặc định của ảnh xem thử — trung tính để thấy rõ chất của bộ style.
	stylePreviewSubject = "a person working at a desk near a window"
	stylePreviewTimeout = 5 * time.Minute
	stylePaletteMax     = 12

	styleNoKeyHint = "chưa cấu hình Gemini API key — mở trang \"Cấu hình & API\", dán khoá Gemini rồi bấm Lưu để xem thử ảnh của bộ style"

	styleDirName    = "styles"
	styleStockMax   = 24 // trần số tư liệu nền của một bộ style
	styleHTMLLimit  = 60 * time.Second
	styleUploadSize = 2 << 30

	// Nội dung mặc định của khung xem trước — mở trần vẫn ra cảnh tử tế.
	stylePreviewTitle = "Tiêu đề cảnh mẫu"
	stylePreviewSub   = "Chữ, màu và phông chữ đúng như khi dựng video"

	// Mặc định phần giao diện video khi tạo bộ style mới (khớp store).
	styleDefBgDeep    = "#0F0A1E"
	styleDefTextMain  = "#F8FAFC"
	styleDefAccent    = "#F59E0B"
	styleDefFont      = `-apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif`
	styleDefSizeTitle = 48
	styleDefSizeBig   = 150
	styleDefSizeBody  = 22
	styleDefVoiceMax  = 180
	styleDefImageMax  = 200
)

// routesStyle — Style Kit: bộ style điều khiển ảnh sinh ra VÀ toàn bộ giao diện
// video (font, cỡ chữ, màu, logo, tư liệu nền).
func (s *Server) routesStyle(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/styles", s.handleStyleList)
	mux.HandleFunc("POST /api/styles", s.handleStyleCreate)
	mux.HandleFunc("PUT /api/styles/{id}", s.handleStyleUpdate)
	mux.HandleFunc("DELETE /api/styles/{id}", s.handleStyleDelete)
	mux.HandleFunc("POST /api/styles/{id}/default", s.handleStyleDefault)
	mux.HandleFunc("POST /api/styles/{id}/preview", s.handleStylePreview)
	mux.HandleFunc("GET /api/styles/{id}/preview.html", s.handleStylePreviewHTML)
	mux.HandleFunc("POST /api/styles/{id}/logo", s.handleStyleLogo)
	mux.HandleFunc("POST /api/styles/{id}/stock", s.handleStyleStockAdd)
	mux.HandleFunc("DELETE /api/styles/{id}/stock", s.handleStyleStockDelete)
}

// styleBody — payload tạo / sửa bộ style. Phần giao diện video dùng con trỏ:
// FE không gửi field nào thì field đó GIỮ NGUYÊN (bản FE cũ chỉ gửi 6 field
// đầu, không được vì thế mà xoá mất cấu hình template).
type styleBody struct {
	Name        string   `json:"name"`
	StylePrompt string   `json:"stylePrompt"`
	Negative    string   `json:"negative"`
	Palette     []string `json:"palette"`
	Theme       string   `json:"theme"`
	IsDefault   bool     `json:"isDefault"`

	BgDeep        *string `json:"bgDeep"`
	TextMain      *string `json:"textMain"`
	Accent        *string `json:"accent"`
	FontHead      *string `json:"fontHead"`
	FontBody      *string `json:"fontBody"`
	SizeTitle     *int    `json:"sizeTitle"`
	SizeBig       *int    `json:"sizeBig"`
	SizeBody      *int    `json:"sizeBody"`
	ChannelName   *string `json:"channelName"`
	LogoPos       *string `json:"logoPos"`
	MaxVoiceChars *int    `json:"maxVoiceChars"`
	MaxImageChars *int    `json:"maxImageChars"`
	BaseTemplate  *string `json:"baseTemplate"`
	CustomHTML    *string `json:"customHtml"`
	// LogoPath chỉ nhận giá trị RỖNG (nút "Bỏ logo"); đường dẫn logo do endpoint
	// tải lên quản lý, không cho payload trỏ tuỳ ý vào file khác.
	LogoPath *string `json:"logoPath"`
}

// handleStyleList — GET /api/styles (bộ đang dùng đứng đầu, sau đó mới nhất trước).
func (s *Server) handleStyleList(w http.ResponseWriter, r *http.Request) {
	list := s.st.StyleKits()
	if list == nil {
		list = []store.StyleKit{}
	}
	writeJSON(w, http.StatusOK, list)
}

// handleStyleCreate — POST /api/styles.
func (s *Server) handleStyleCreate(w http.ResponseWriter, r *http.Request) {
	body, ok := readStyleBody(w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		httpErr(w, http.StatusBadRequest, "thiếu tên bộ style")
		return
	}
	k := store.StyleKit{
		Name:        name,
		StylePrompt: strings.TrimSpace(body.StylePrompt),
		Negative:    strings.TrimSpace(body.Negative),
		Palette:     normalizePalette(body.Palette),
		Theme:       normalizeStyleTheme(body.Theme),
		// Bộ đầu tiên luôn là bộ đang dùng — hệ thống không được thiếu bộ mặc định.
		IsDefault: body.IsDefault || len(s.st.StyleKits()) == 0,
	}
	applyStyleDesign(&k, body)
	fillStyleDesignDefaults(&k)
	s.st.SaveStyleKit(&k)
	s.Log("info", "style", fmt.Sprintf("Đã tạo bộ style %q", k.Name))
	writeJSON(w, http.StatusOK, k)
}

// handleStyleUpdate — PUT /api/styles/{id}.
func (s *Server) handleStyleUpdate(w http.ResponseWriter, r *http.Request) {
	cur, ok := s.styleKit(w, r)
	if !ok {
		return
	}
	body, ok := readStyleBody(w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		httpErr(w, http.StatusBadRequest, "thiếu tên bộ style")
		return
	}
	cur.Name = name
	cur.StylePrompt = strings.TrimSpace(body.StylePrompt)
	cur.Negative = strings.TrimSpace(body.Negative)
	cur.Palette = normalizePalette(body.Palette)
	cur.Theme = normalizeStyleTheme(body.Theme)
	// Bộ đang mặc định không tự bỏ mặc định khi sửa — muốn đổi thì chọn bộ khác
	// làm mặc định, tránh trạng thái không bộ nào được dùng.
	cur.IsDefault = cur.IsDefault || body.IsDefault
	applyStyleDesign(&cur, body)
	s.clearStyleLogo(&cur, body)

	s.st.SaveStyleKit(&cur)
	s.Log("info", "style", fmt.Sprintf("Đã cập nhật bộ style %q", cur.Name))
	writeJSON(w, http.StatusOK, cur)
}

// handleStyleDelete — DELETE /api/styles/{id}; xoá bộ đang mặc định thì tự đặt
// bộ còn lại mới nhất làm mặc định.
func (s *Server) handleStyleDelete(w http.ResponseWriter, r *http.Request) {
	k, ok := s.styleKit(w, r)
	if !ok {
		return
	}
	s.st.DeleteStyleKit(k.ID)
	if err := os.Remove(s.stylePreviewPath(k.ID)); err != nil && !os.IsNotExist(err) {
		s.Log("warn", "style", fmt.Sprintf("Không xoá được ảnh xem thử của bộ style %q: %v", k.Name, err))
	}
	// Logo, tư liệu nền và khung hình nền đã trích đều đi theo bộ style.
	if p := s.styleDataPath(k.LogoPath); p != "" {
		_ = os.Remove(p)
	}
	for _, rel := range k.StockPaths {
		if p := s.styleDataPath(rel); p != "" {
			_ = os.Remove(p)
		}
	}
	_ = os.RemoveAll(s.styleBgCacheDir(k.ID))
	s.Log("info", "style", fmt.Sprintf("Đã xoá bộ style %q", k.Name))
	if k.IsDefault {
		s.promoteNewestStyle()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleStyleDefault — POST /api/styles/{id}/default.
func (s *Server) handleStyleDefault(w http.ResponseWriter, r *http.Request) {
	k, ok := s.styleKit(w, r)
	if !ok {
		return
	}
	k.IsDefault = true
	s.st.SaveStyleKit(&k)
	s.Log("info", "style", fmt.Sprintf("Đang dùng bộ style %q cho mọi ảnh sinh ra", k.Name))
	writeJSON(w, http.StatusOK, k)
}

// handleStylePreview — POST /api/styles/{id}/preview: sinh ảnh mẫu của bộ style.
func (s *Server) handleStylePreview(w http.ResponseWriter, r *http.Request) {
	k, ok := s.styleKit(w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(s.st.Settings().GeminiAPIKey) == "" {
		httpErr(w, http.StatusBadRequest, "%s", styleNoKeyHint)
		return
	}
	var body struct {
		Subject string `json:"subject"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(r, &body); err != nil {
			httpErr(w, http.StatusBadRequest, "%s", err)
			return
		}
	}
	subject := strings.TrimSpace(body.Subject)
	if subject == "" {
		subject = stylePreviewSubject
	}
	prompt := stylekit.ApplyKit(k, subject)
	dst := s.stylePreviewPath(k.ID)

	j := s.Jobs.Submit("style_preview", "", "Xem thử bộ style: "+shortText(k.Name, 40),
		func(upd func(float64, string)) (string, error) {
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return "", fmt.Errorf("không tạo được thư mục ảnh xem thử: %w", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), stylePreviewTimeout)
			defer cancel()
			upd(20, "Đang sinh ảnh mẫu bằng Gemini…")
			if err := gemini.NewFromSettings(s.st).GenerateImage(ctx, prompt, dst); err != nil {
				return "", err
			}
			return s.toolRelPath(dst), nil
		})
	writeJSON(w, http.StatusOK, j)
}

// handleStylePreviewHTML — GET /api/styles/{id}/preview.html: trả HTML SỐNG của
// một cảnh, dựng bằng ĐÚNG bộ dựng dùng khi render video (không có template thứ
// hai) nên chỉnh gì trong bộ style là thấy nấy. FE nhúng bằng iframe.
func (s *Server) handleStylePreviewHTML(w http.ResponseWriter, r *http.Request) {
	k, ok := s.styleKit(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	sc := previewScene(q)
	cfg := htmlvideo.Config{
		Aspect: strings.TrimSpace(q.Get("aspect")),
		Theme:  k.Theme,
		// safe=1 vẽ khung nhắc vùng bị ứng dụng xem video che, để căn bố cục.
		SafeGuides: q.Get("safe") == "1",
	}

	ctx, cancel := context.WithTimeout(r.Context(), styleHTMLLimit)
	defer cancel()
	html, err := htmlvideo.PreviewHTML(ctx, s.st, &k, sc, cfg, s.styleBgCacheDir(k.ID))
	if err != nil {
		httpErr(w, http.StatusBadRequest, "không dựng được khung xem trước: %v", err)
		return
	}
	if at, parseErr := strconv.ParseFloat(q.Get("previewAt"), 64); parseErr == nil && at >= 0 && at <= 60 {
		html = appendPreviewSeek(html, at)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src data: blob:; media-src data: blob:; font-src data:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'none'; frame-ancestors 'self'")
	// Xem trước phải luôn là bản mới nhất — iframe không được dùng bản cache.
	w.Header().Set("Cache-Control", "no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

func appendPreviewSeek(html string, at float64) string {
	script := fmt.Sprintf(`<script>addEventListener('load',function(){if(typeof window.seek==='function')window.seek(%g)})</script>`, at)
	lower := strings.ToLower(html)
	if i := strings.LastIndex(lower, "</body>"); i >= 0 {
		return html[:i] + script + html[i:]
	}
	return html + script
}

// handleStyleLogo — POST /api/styles/{id}/logo: multipart "files" (1 ảnh) →
// data/styles/<id>-logo.png.
func (s *Server) handleStyleLogo(w http.ResponseWriter, r *http.Request) {
	k, ok := s.styleKit(w, r)
	if !ok {
		return
	}
	fhs, ok := s.styleUploads(w, r, "chưa chọn ảnh logo (field \"files\")")
	if !ok {
		return
	}
	tmp, err := s.saveStyleUpload(fhs[0])
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "%s", err)
		return
	}
	defer os.Remove(tmp)

	ctx, cancel := context.WithTimeout(r.Context(), styleHTMLLimit)
	defer cancel()
	dst := filepath.Join(s.DataDir, styleDirName, k.ID+"-logo.png")
	if err := s.storeStyleLogo(ctx, tmp, dst); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	k.LogoPath = path.Join(styleDirName, k.ID+"-logo.png")
	s.st.SaveStyleKit(&k)
	s.Log("info", "style", fmt.Sprintf("Đã cập nhật logo cho bộ style %q", k.Name))
	writeJSON(w, http.StatusOK, k)
}

// handleStyleStockAdd — POST /api/styles/{id}/stock: multipart "files" (nhiều
// ảnh/video) → data/styles/<id>-stock-<n>.<ext>, thêm vào thư viện tư liệu nền.
func (s *Server) handleStyleStockAdd(w http.ResponseWriter, r *http.Request) {
	k, ok := s.styleKit(w, r)
	if !ok {
		return
	}
	fhs, ok := s.styleUploads(w, r, "chưa chọn tư liệu nền (field \"files\")")
	if !ok {
		return
	}
	if len(k.StockPaths) >= styleStockMax {
		httpErr(w, http.StatusBadRequest, "thư viện tư liệu nền đã đủ %d mục — xoá bớt trước khi thêm", styleStockMax)
		return
	}

	added, fails := 0, make([]string, 0, len(fhs))
	for _, fh := range fhs {
		if len(k.StockPaths) >= styleStockMax {
			break
		}
		rel, err := s.storeStyleStock(k.ID, len(k.StockPaths), fh)
		if err != nil {
			fails = append(fails, fmt.Sprintf("%s: %v", sanitizeFileName(fh.Filename), err))
			continue
		}
		k.StockPaths = append(k.StockPaths, rel)
		added++
	}
	if added == 0 {
		httpErr(w, http.StatusBadRequest, "không thêm được tư liệu nền nào — %s", strings.Join(fails, " | "))
		return
	}
	s.st.SaveStyleKit(&k)
	// Khung hình nền đã trích của bộ style cũ không còn đúng nữa.
	_ = os.RemoveAll(s.styleBgCacheDir(k.ID))
	s.Log("info", "style", fmt.Sprintf("Đã thêm %d tư liệu nền cho bộ style %q", added, k.Name))
	if len(fails) > 0 {
		s.Log("warn", "style", fmt.Sprintf("Bỏ qua %d tư liệu nền: %s", len(fails), strings.Join(fails, " | ")))
	}
	writeJSON(w, http.StatusOK, k)
}

// handleStyleStockDelete — DELETE /api/styles/{id}/stock?path=…: gỡ 1 tư liệu
// nền khỏi thư viện và xoá file. Chỉ chấp nhận đường dẫn ĐANG có trong bộ style
// (chặn xoá lung tung ra ngoài thư mục dữ liệu).
func (s *Server) handleStyleStockDelete(w http.ResponseWriter, r *http.Request) {
	k, ok := s.styleKit(w, r)
	if !ok {
		return
	}
	target := strings.TrimSpace(r.URL.Query().Get("path"))
	if target == "" {
		httpErr(w, http.StatusBadRequest, "thiếu tham số path — cần đường dẫn tư liệu nền cần gỡ")
		return
	}
	rest := make([]string, 0, len(k.StockPaths))
	found := false
	for _, p := range k.StockPaths {
		if !found && p == target {
			found = true
			continue
		}
		rest = append(rest, p)
	}
	if !found {
		httpErr(w, http.StatusNotFound, "bộ style %q không có tư liệu nền %q", k.Name, target)
		return
	}
	if p := s.styleDataPath(target); p != "" {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			s.Log("warn", "style", fmt.Sprintf("Không xoá được file tư liệu nền %s: %v", target, err))
		}
	}
	k.StockPaths = rest
	s.st.SaveStyleKit(&k)
	_ = os.RemoveAll(s.styleBgCacheDir(k.ID))
	s.Log("info", "style", fmt.Sprintf("Đã gỡ tư liệu nền khỏi bộ style %q", k.Name))
	writeJSON(w, http.StatusOK, k)
}

// ---------- helpers ----------

// styleKit lấy bộ style theo path param, tự trả 404 tiếng Việt nếu không có.
func (s *Server) styleKit(w http.ResponseWriter, r *http.Request) (store.StyleKit, bool) {
	id := r.PathValue("id")
	k, ok := s.st.StyleKit(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy bộ style %q", id)
		return store.StyleKit{}, false
	}
	return k, true
}

// stylePreviewPath — data/styles/<id>-preview.png.
func (s *Server) stylePreviewPath(id string) string {
	return filepath.Join(s.DataDir, "styles", id+"-preview.png")
}

// readStyleBody đọc payload tạo/sửa; đã ghi lỗi HTTP nếu body hỏng.
func readStyleBody(w http.ResponseWriter, r *http.Request) (styleBody, bool) {
	var body styleBody
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return styleBody{}, false
	}
	return body, true
}

// promoteNewestStyle đặt bộ còn lại mới nhất làm mặc định — luôn phải có một bộ
// đang dùng, nếu không mọi ảnh sinh ra sẽ mất tính đồng nhất.
func (s *Server) promoteNewestStyle() {
	rest := s.st.StyleKits()
	if len(rest) == 0 {
		return
	}
	next := rest[0]
	for _, k := range rest {
		if k.UpdatedAt.After(next.UpdatedAt) {
			next = k
		}
	}
	next.IsDefault = true
	s.st.SaveStyleKit(&next)
	s.Log("info", "style", fmt.Sprintf("Chuyển sang dùng bộ style %q", next.Name))
}

// normalizeStyleTheme ép theme về giá trị hợp lệ (vivid | dark | light).
func normalizeStyleTheme(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "dark":
		return "dark"
	case "light":
		return "light"
	default:
		return "vivid"
	}
}

// normalizePalette bỏ mã màu rỗng và giới hạn số lượng màu.
func normalizePalette(list []string) []string {
	out := make([]string, 0, len(list))
	for _, c := range list {
		if c = strings.TrimSpace(c); c == "" {
			continue
		}
		out = append(out, c)
		if len(out) >= stylePaletteMax {
			break
		}
	}
	return out
}

// ---------- phần giao diện video ----------

// applyStyleDesign ghi các field giao diện video mà FE CÓ gửi; field không gửi
// (con trỏ nil) được giữ nguyên để bản FE cũ không xoá mất cấu hình template.
func applyStyleDesign(k *store.StyleKit, b styleBody) {
	setStyleStr(&k.BgDeep, b.BgDeep, nil)
	setStyleStr(&k.TextMain, b.TextMain, nil)
	setStyleStr(&k.Accent, b.Accent, nil)
	setStyleStr(&k.FontHead, b.FontHead, nil)
	setStyleStr(&k.FontBody, b.FontBody, nil)
	setStyleStr(&k.ChannelName, b.ChannelName, nil)
	setStyleStr(&k.LogoPos, b.LogoPos, normalizeLogoPos)
	setStyleStr(&k.BaseTemplate, b.BaseTemplate, normalizeBaseTemplate)
	if b.CustomHTML != nil {
		k.CustomHTML = *b.CustomHTML
	}
	setStyleInt(&k.SizeTitle, b.SizeTitle, 400)
	setStyleInt(&k.SizeBig, b.SizeBig, 600)
	setStyleInt(&k.SizeBody, b.SizeBody, 400)
	setStyleInt(&k.MaxVoiceChars, b.MaxVoiceChars, 2000)
	setStyleInt(&k.MaxImageChars, b.MaxImageChars, 2000)
}

// fillStyleDesignDefaults bù mặc định cho bộ style vừa tạo — thiếu font/cỡ chữ
// thì FE không có gì để hiển thị và khung xem trước sẽ trống trơn.
func fillStyleDesignDefaults(k *store.StyleKit) {
	if k.BgDeep == "" {
		k.BgDeep = styleDefBgDeep
	}
	if k.TextMain == "" {
		k.TextMain = styleDefTextMain
	}
	if k.Accent == "" {
		if len(k.Palette) > 0 {
			k.Accent = k.Palette[0]
		} else {
			k.Accent = styleDefAccent
		}
	}
	if k.FontHead == "" {
		k.FontHead = styleDefFont
	}
	if k.FontBody == "" {
		k.FontBody = styleDefFont
	}
	if k.SizeTitle == 0 {
		k.SizeTitle = styleDefSizeTitle
	}
	if k.SizeBig == 0 {
		k.SizeBig = styleDefSizeBig
	}
	if k.SizeBody == 0 {
		k.SizeBody = styleDefSizeBody
	}
	if k.LogoPos == "" {
		k.LogoPos = "left"
	}
	if k.BaseTemplate == "" {
		k.BaseTemplate = "builtin"
	}
	if k.MaxVoiceChars == 0 {
		k.MaxVoiceChars = styleDefVoiceMax
	}
	if k.MaxImageChars == 0 {
		k.MaxImageChars = styleDefImageMax
	}
}

// clearStyleLogo xử lý nút "Bỏ logo": FE gửi logoPath rỗng thì gỡ logo và xoá
// file. Giá trị KHÁC rỗng bị bỏ qua — chỉ endpoint tải lên mới đặt được logo.
func (s *Server) clearStyleLogo(k *store.StyleKit, b styleBody) {
	if b.LogoPath == nil || strings.TrimSpace(*b.LogoPath) != "" || k.LogoPath == "" {
		return
	}
	if p := s.styleDataPath(k.LogoPath); p != "" {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			s.Log("warn", "style", fmt.Sprintf("Không xoá được file logo %s: %v", k.LogoPath, err))
		}
	}
	k.LogoPath = ""
	s.Log("info", "style", fmt.Sprintf("Đã bỏ logo của bộ style %q", k.Name))
}

// setStyleStr ghi giá trị chuỗi khi FE có gửi (nil = không gửi = giữ nguyên).
func setStyleStr(dst *string, v *string, norm func(string) string) {
	if v == nil {
		return
	}
	out := strings.TrimSpace(*v)
	if norm != nil {
		out = norm(out)
	}
	*dst = out
}

// setStyleInt ghi giá trị số khi FE có gửi, chặn dải cho khỏi vỡ bố cục.
func setStyleInt(dst *int, v *int, max int) {
	if v == nil {
		return
	}
	out := *v
	if out < 0 {
		out = 0
	}
	if out > max {
		out = max
	}
	*dst = out
}

// normalizeLogoPos ép vị trí logo về giá trị hợp lệ (left | center | right).
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

// normalizeBaseTemplate ép nền tảng giao diện về builtin | custom.
func normalizeBaseTemplate(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), "custom") {
		return "custom"
	}
	return "builtin"
}

// ---------- xem trước & tư liệu ----------

// previewScene dựng cảnh mẫu cho khung xem trước từ query; thiếu tham số thì
// dùng nội dung mặc định để mở trần vẫn ra cảnh đẹp.
func previewScene(q map[string][]string) htmlvideo.Scene {
	get := func(key string) (string, bool) {
		v, ok := q[key]
		if !ok || len(v) == 0 {
			return "", false
		}
		return strings.TrimSpace(v[0]), true
	}
	tpl, _ := get("template")
	sc := htmlvideo.Scene{Template: tpl, Duration: 3}
	if v, ok := get("title"); ok {
		sc.Title = v
	} else {
		sc.Title = stylePreviewTitle
	}
	if v, ok := get("subtitle"); ok {
		sc.Subtitle = v
	} else {
		sc.Subtitle = stylePreviewSub
	}
	switch strings.ToLower(tpl) {
	case "bullets":
		sc.Bullets = []string{"Ý chính thứ nhất của cảnh", "Ý chính thứ hai ngắn gọn", "Ý chính thứ ba chốt lại"}
	case "keys":
		// keys dùng title làm từ khoá chính, bullets làm các từ khoá liên quan.
		sc.Bullets = []string{"SJC", "Nhẫn trơn", "Ngân hàng", "Tỷ giá", "Đầu tư"}
	case "chart":
		sc.Chart = []htmlvideo.ChartItem{{Label: "Trước", Value: 32}, {Label: "Sau", Value: 78}}
	case "code":
		sc.Code = "func main() {\n\tfmt.Println(\"Biz Studio\")\n}"
	}
	return sc
}

// styleDataPath đổi đường dẫn tương đối DataDir → tuyệt đối, chặn thoát ra
// ngoài thư mục dữ liệu; không hợp lệ → "".
func (s *Server) styleDataPath(rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return ""
	}
	p := filepath.Clean(filepath.Join(s.DataDir, filepath.FromSlash(rel)))
	if !strings.HasPrefix(p, filepath.Clean(s.DataDir)+string(os.PathSeparator)) {
		return ""
	}
	return p
}

// styleBgCacheDir — nơi chứa khung hình nền đã trích của một bộ style.
func (s *Server) styleBgCacheDir(id string) string {
	return filepath.Join(s.DataDir, "tmp", "stylebg", id)
}

// styleUploads đọc form multipart và trả danh sách file; đã ghi lỗi HTTP nếu hỏng.
func (s *Server) styleUploads(w http.ResponseWriter, r *http.Request, missing string) ([]*multipart.FileHeader, bool) {
	if err := r.ParseMultipartForm(styleUploadSize); err != nil {
		httpErr(w, http.StatusBadRequest, "không đọc được form upload: %v", err)
		return nil, false
	}
	if r.MultipartForm == nil || len(r.MultipartForm.File["files"]) == 0 {
		httpErr(w, http.StatusBadRequest, "%s", missing)
		return nil, false
	}
	return r.MultipartForm.File["files"], true
}

// saveStyleUpload lưu file upload vào data/tmp để xử lý; trả đường dẫn file tạm.
func (s *Server) saveStyleUpload(fh *multipart.FileHeader) (string, error) {
	dir := filepath.Join(s.DataDir, "tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("không tạo được thư mục tạm: %w", err)
	}
	tmp := filepath.Join(dir, s.st.NewID("stysrc")+filepath.Ext(sanitizeFileName(fh.Filename)))
	if _, err := saveMultipartFile(fh, tmp); err != nil {
		return "", err
	}
	return tmp, nil
}

// storeStyleLogo chuyển ảnh logo sang PNG tại dst bằng ffmpeg (cũng là bước
// kiểm tra file có thật là ảnh). Ghi ra file tạm rồi mới thay file đích để logo
// cũ không mất khi lỗi.
func (s *Server) storeStyleLogo(ctx context.Context, src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("không tạo được thư mục bộ style: %w", err)
	}
	tmp := dst + ".part.png"
	defer os.Remove(tmp)

	if _, se, err := util.RunErr(ctx, "ffmpeg", "-y", "-i", src, "-frames:v", "1", tmp); err != nil {
		return fmt.Errorf("không đọc được ảnh logo (chỉ nhận file ảnh: png, jpg, webp, heic…): %v — %s",
			err, cloneStderrTail(se))
	}
	if fi, err := os.Stat(tmp); err != nil || fi.Size() == 0 {
		return fmt.Errorf("ảnh logo rỗng hoặc không đọc được")
	}
	if err := os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("không ghi được logo: %w", err)
	}
	return nil
}

// storeStyleStock lưu 1 tư liệu nền (ảnh hoặc video) vào data/styles và trả
// đường dẫn tương đối DataDir. File phải đọc được bằng ffprobe — tư liệu hỏng
// lọt vào đây sẽ làm hỏng nền của cả video.
func (s *Server) storeStyleStock(kitID string, seq int, fh *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(sanitizeFileName(fh.Filename)))
	if ext == "" {
		ext = ".png"
	}
	dir := filepath.Join(s.DataDir, styleDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("không tạo được thư mục bộ style: %w", err)
	}
	name := fmt.Sprintf("%s-stock-%d%s", kitID, seq, ext)
	for i := seq; ; i++ {
		name = fmt.Sprintf("%s-stock-%d%s", kitID, i, ext)
		if _, err := os.Stat(filepath.Join(dir, name)); os.IsNotExist(err) {
			break
		}
	}
	dst := filepath.Join(dir, name)
	if _, err := saveMultipartFile(fh, dst); err != nil {
		return "", err
	}
	if _, err := media.Probe(dst); err != nil {
		_ = os.Remove(dst)
		return "", fmt.Errorf("không đọc được tư liệu (chỉ nhận ảnh hoặc video): %v", err)
	}
	return path.Join(styleDirName, name), nil
}
