// Package recap — biến PHIM DÀI thành VIDEO KỂ CHUYỆN: chia phim theo chuyển
// cảnh, AI xem khung hình thật của từng cảnh rồi viết lời dẫn, đọc bằng giọng
// của máy, ghép lại thành video có lời kể đè lên phim (tiếng gốc tự né lời).
//
// Ba nguyên tắc — rút từ việc mổ những công cụ cùng loại làm SAI:
//  1. AI phải NHÌN khung hình thật của cảnh; chỉ đưa mốc thời gian thì AI viết
//     mù, lời dẫn không dính gì tới nội dung phim.
//  2. Mốc thời gian tính từ THỜI LƯỢNG GIỌNG ĐO THẬT, không ước theo số chữ.
//  3. Lời dài hơn cảnh thì tăng tốc giọng có trần, quá trần thì ĐÓNG BĂNG khung
//     hình cuối kéo dài cảnh — tuyệt đối không cắt cụt lời đang đọc.
package recap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/media"
	"bizstudio/internal/store"
)

// Các phong cách lời dẫn.
const (
	StyleKeChuyen = "ke-chuyen" // thuật lại như kể cho người chưa xem
	StyleReview   = "review"    // bình phim, có nhận xét cá nhân
	StyleTomTat   = "tom-tat"   // tóm tắt nhanh, nhịp dồn
)

// Scene — một cảnh của phim kèm lời dẫn.
type Scene struct {
	Index int     `json:"index"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Frame string  `json:"frame"` // tương đối DataDir
	Text  string  `json:"text"`  // lời dẫn — AI viết, người dùng sửa được
}

// Manifest — hồ sơ một phiên kể chuyện, lưu tại data/recap/<id>/manifest.json.
// Đây là chỗ người dùng sửa lời từng cảnh trước khi render — và cũng là nguồn
// dữ liệu cho bước xuất sang trình dựng ngoài.
type Manifest struct {
	ID     string  `json:"id"`
	Source string  `json:"source"` // video nguồn, tương đối DataDir
	Style  string  `json:"style"`
	Scenes []Scene `json:"scenes"`

	// Merged — số lần gộp cảnh khi ép trần maxScenes; >0 thì giao diện phải
	// nói rõ "đã gộp N lần", không để người dùng tưởng phim vốn ít cảnh vậy.
	Merged int `json:"merged"`
	// NarrationNote — vì sao lời dẫn trống (chưa có khoá AI...), rỗng = AI đã viết.
	NarrationNote string    `json:"narrationNote,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// AnalyzeOpts — tham số bước phân tích.
type AnalyzeOpts struct {
	Threshold float64 // 0 = 0.35
	MinScene  float64 // 0 = 2s
	MaxScenes int     // 0 = không ép trần
	Style     string
	Narration string // "ai" (mặc định) | "none" — none: để trống cho người dùng tự viết
}

// Dir trả thư mục phiên: <dataDir>/recap/<id>.
func Dir(dataDir, id string) string { return filepath.Join(dataDir, "recap", id) }

// ManifestPath — đường dẫn manifest trong thư mục phiên.
func ManifestPath(dataDir, id string) string { return filepath.Join(Dir(dataDir, id), "manifest.json") }

// Analyze chia phim thành cảnh, trích khung hình đại diện và (nếu bật) nhờ AI
// viết lời dẫn. srcAbs là đường dẫn tuyệt đối video nguồn; srcRel là đường dẫn
// tương đối DataDir để ghi vào manifest.
func Analyze(ctx context.Context, st *store.Store, srcAbs, srcRel, id string,
	opt AnalyzeOpts, upd func(float64, string)) (*Manifest, error) {

	if upd == nil {
		upd = func(float64, string) {}
	}
	style := normalizeStyle(opt.Style)

	upd(5, "Bước 1/3: dò chuyển cảnh (đọc toàn bộ phim, phim dài sẽ lâu)…")
	scenes, err := media.DetectScenes(ctx, srcAbs, opt.Threshold, opt.MinScene)
	if err != nil {
		return nil, err
	}
	merged := 0
	if opt.MaxScenes > 0 {
		scenes, merged = media.MergeToMaxScenes(scenes, opt.MaxScenes)
	}
	upd(35, fmt.Sprintf("Thấy %d cảnh%s", len(scenes), mergedNote(merged)))

	upd(40, "Bước 2/3: trích khung hình đại diện từng cảnh…")
	frameDir := filepath.Join(Dir(st.DataDir, id), "frames")
	if err := media.ExtractSceneFrames(ctx, srcAbs, scenes, frameDir, 640); err != nil {
		return nil, err
	}

	m := &Manifest{
		ID: id, Source: srcRel, Style: style, Merged: merged,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	for _, s := range scenes {
		m.Scenes = append(m.Scenes, Scene{
			Index: s.Index, Start: s.Start, End: s.End,
			Frame: relOf(st.DataDir, s.Frame),
		})
	}

	if opt.Narration != "none" {
		upd(55, "Bước 3/3: AI xem khung hình từng cảnh và viết lời dẫn…")
		if err := Narrate(ctx, st, m, upd); err != nil {
			// Không có AI thì phiên vẫn dùng được: người dùng tự viết lời.
			m.NarrationNote = err.Error()
		}
	} else {
		m.NarrationNote = "Bạn chọn tự viết lời — điền lời dẫn cho từng cảnh rồi bấm Dựng video."
	}

	if err := m.Save(st.DataDir); err != nil {
		return nil, err
	}
	upd(98, fmt.Sprintf("Xong: %d cảnh%s", len(m.Scenes), mergedNote(merged)))
	return m, nil
}

// Save ghi manifest xuống đĩa (ghi tạm rồi đổi tên).
func (m *Manifest) Save(dataDir string) error {
	m.UpdatedAt = time.Now()
	p := ManifestPath(dataDir, m.ID)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("tạo thư mục phiên: %w", err)
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("mã hoá manifest: %w", err)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("ghi manifest: %w", err)
	}
	return os.Rename(tmp, p)
}

// Load đọc manifest từ đường dẫn tuyệt đối.
func Load(path string) (*Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("không đọc được manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("manifest không đúng định dạng: %w", err)
	}
	if m.ID == "" || len(m.Scenes) == 0 {
		return nil, fmt.Errorf("manifest thiếu dữ liệu (id hoặc danh sách cảnh)")
	}
	return &m, nil
}

func normalizeStyle(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case StyleReview:
		return StyleReview
	case StyleTomTat:
		return StyleTomTat
	default:
		return StyleKeChuyen
	}
}

func mergedNote(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf(" (đã gộp %d lần cho vừa trần số cảnh)", n)
}

func relOf(dataDir, abs string) string {
	if r, err := filepath.Rel(dataDir, abs); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return abs
}
