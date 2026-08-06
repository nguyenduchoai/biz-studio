// Package veo — sinh video bằng Google Veo qua Gemini API.
//
// Khác mọi module khác trong Biz Studio, đây là tính năng TRẢ PHÍ TÍNH THEO
// GIÂY trên khoá riêng của người dùng. Vì vậy gói này luôn kèm ước tính chi phí
// và giao diện bắt buộc hiện con số đó trước khi bấm tạo — người dùng không bao
// giờ bị trừ tiền vì một cú bấm nhầm.
package veo

import (
	"fmt"
	"strings"
)

// Model — một model Veo kèm bảng giá theo độ phân giải (USD mỗi giây video).
type Model struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Desc       string             `json:"desc"`
	Deprecated bool               `json:"deprecated"`
	PricePerS  map[string]float64 `json:"pricePerSec"` // "720p" | "1080p" | "4k"
}

// Bảng giá lấy từ trang giá chính thức của Gemini API (USD/giây, đã gồm tiếng).
// Giá có thể đổi — con số hiện trên giao diện là ƯỚC TÍNH, hoá đơn thật do
// Google tính.
var models = []Model{
	{
		ID: "veo-3.1-generate-preview", Name: "Veo 3.1 — chuẩn",
		Desc:      "Chất lượng cao nhất, có tiếng. Dùng cho cảnh chủ đạo.",
		PricePerS: map[string]float64{"720p": 0.40, "1080p": 0.40, "4k": 0.60},
	},
	{
		ID: "veo-3.1-fast-generate-preview", Name: "Veo 3.1 — nhanh",
		Desc:      "Rẻ hơn 4 lần, nhanh hơn. Hợp để thử ý tưởng và làm cảnh phụ.",
		PricePerS: map[string]float64{"720p": 0.10, "1080p": 0.12, "4k": 0.30},
	},
	{
		ID: "veo-3.1-lite-generate-preview", Name: "Veo 3.1 — lite",
		Desc:      "Rẻ nhất. Hợp để dò prompt trước khi chạy bản chuẩn.",
		PricePerS: map[string]float64{"720p": 0.05, "1080p": 0.08},
	},
}

// Không liệt kê veo-3.0-*: đã kiểm bằng cách hỏi thẳng API models.list — Google
// đã GỠ HẲN hai model đó, chọn vào là nhận lỗi 404 sau khi người dùng đã chờ.
// Bảng trên chỉ dùng để TRA GIÁ; danh sách model thật lấy từ API để không bao
// giờ chào một model không còn tồn tại.

// DefaultModel — model dùng khi người dùng chưa chọn.
const DefaultModel = "veo-3.1-fast-generate-preview"

// Models trả bảng model (bản sao).
func Models() []Model {
	out := make([]Model, len(models))
	copy(out, models)
	return out
}

// FindModel tìm model theo id.
func FindModel(id string) (Model, bool) {
	for _, m := range models {
		if strings.EqualFold(m.ID, id) {
			return m, true
		}
	}
	return Model{}, false
}

// EstimateUSD ước tính chi phí một lần tạo. Không tra được giá → 0 kèm lý do,
// giao diện sẽ nói rõ "không ước tính được" thay vì hiện số bịa.
func EstimateUSD(modelID, resolution string, seconds, count int) (float64, error) {
	m, ok := FindModel(modelID)
	if !ok {
		return 0, fmt.Errorf("không có model %q", modelID)
	}
	p, ok := m.PricePerS[normalizeResolution(resolution)]
	if !ok {
		return 0, fmt.Errorf("model %s không hỗ trợ độ phân giải %s", m.Name, resolution)
	}
	if seconds <= 0 {
		seconds = DefaultDuration
	}
	if count <= 0 {
		count = 1
	}
	return p * float64(seconds) * float64(count), nil
}

// ---------- chuẩn hoá tham số ----------

const (
	// DefaultDuration — Veo chỉ nhận 4, 6 hoặc 8 giây.
	DefaultDuration   = 8
	DefaultResolution = "720p"
	DefaultAspect     = "9:16"
)

// AllowedDurations — độ dài Veo chấp nhận.
var AllowedDurations = []int{4, 6, 8}

func normalizeResolution(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1080p":
		return "1080p"
	case "4k":
		return "4k"
	default:
		return "720p"
	}
}

func normalizeAspect(v string) string {
	if strings.TrimSpace(v) == "16:9" {
		return "16:9"
	}
	return "9:16"
}

// normalizeDuration ép về đúng một trong các mốc Veo nhận; số lạ thì lấy mốc
// gần nhất chứ không để API trả lỗi sau khi người dùng đã chờ.
//
// v <= 0 nghĩa là KHÔNG khai báo, không phải "ngắn nhất": phải trả mặc định 8
// giây (đúng mặc định của Veo). Nếu để rơi vào phép tìm mốc gần nhất thì 0 sẽ
// hoá 4 giây và người gọi nhận clip ngắn hơn họ tưởng.
func normalizeDuration(v int) int {
	if v <= 0 {
		return DefaultDuration
	}
	best, bestGap := DefaultDuration, 1<<30
	for _, d := range AllowedDurations {
		gap := d - v
		if gap < 0 {
			gap = -gap
		}
		if gap < bestGap {
			best, bestGap = d, gap
		}
	}
	return best
}

// checkCombo chặn các tổ hợp Veo không nhận, báo trước khi tốn tiền.
func checkCombo(resolution string, seconds int) error {
	r := normalizeResolution(resolution)
	if (r == "1080p" || r == "4k") && seconds != 8 {
		return fmt.Errorf("%s chỉ tạo được clip 8 giây — hãy chọn 8 giây hoặc hạ về 720p", r)
	}
	return nil
}
