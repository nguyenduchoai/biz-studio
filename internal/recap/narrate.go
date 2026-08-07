package recap

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"bizstudio/internal/gemini"
	"bizstudio/internal/store"
)

// ---------- AI viết lời dẫn: PHẢI nhìn khung hình thật ----------
//
// Mỗi lượt gọi gửi kèm ảnh đại diện của một nhóm cảnh. Chỉ đưa mốc thời gian
// mà không đưa hình thì AI viết mù — lời dẫn nghe xuôi tai nhưng không dính gì
// tới nội dung phim.

// narrateBatch — số cảnh mỗi lượt gọi AI. Ảnh 640px ~60-120KB, trần inline của
// Gemini 18MB nên 8 ảnh/lượt còn rất xa trần; chia lượt để phim trăm cảnh không
// dồn thành một request khổng lồ.
const narrateBatch = 8

// stylePrompt mô tả giọng văn cho từng phong cách.
func stylePrompt(style string) string {
	switch style {
	case StyleReview:
		return "Bạn là người BÌNH PHIM: kể lại kèm nhận xét, khen chê thẳng, thi thoảng chêm góc nhìn cá nhân."
	case StyleTomTat:
		return "Bạn TÓM TẮT PHIM nhịp nhanh: câu ngắn, dồn dập, chỉ giữ diễn biến chính."
	default:
		return "Bạn là người KỂ CHUYỆN PHIM cho người chưa xem: giọng cuốn hút, giữ mạch, không bình luận ngoài lề."
	}
}

// Narrate nhờ AI viết lời dẫn cho các cảnh CHƯA có lời trong manifest.
// Chỉ ghi vào Scene.Text đang trống — không đè lời người dùng đã sửa.
func Narrate(ctx context.Context, st *store.Store, m *Manifest, upd func(float64, string)) error {
	if upd == nil {
		upd = func(float64, string) {}
	}
	cli := gemini.NewFromSettings(st)
	if strings.TrimSpace(cli.Key) == "" {
		return fmt.Errorf("chưa có khoá Gemini (Cấu hình & API) — AI cần nhìn khung hình nên phải có model thị giác; bạn vẫn có thể tự viết lời từng cảnh")
	}

	var pending []int
	for i := range m.Scenes {
		if strings.TrimSpace(m.Scenes[i].Text) == "" {
			pending = append(pending, i)
		}
	}
	if len(pending) == 0 {
		return nil
	}

	for off := 0; off < len(pending); off += narrateBatch {
		end := off + narrateBatch
		if end > len(pending) {
			end = len(pending)
		}
		batch := pending[off:end]
		upd(55+float64(off)/float64(len(pending))*40,
			fmt.Sprintf("AI đang viết lời cảnh %d–%d / %d…", batch[0]+1, batch[len(batch)-1]+1, len(m.Scenes)))
		if err := narrateOnce(ctx, st, cli, m, batch); err != nil {
			return err
		}
	}
	return nil
}

// narrateOnce gọi AI cho một nhóm cảnh, gán lời vào manifest.
func narrateOnce(ctx context.Context, st *store.Store, cli *gemini.Client, m *Manifest, idx []int) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", stylePrompt(m.Style))
	b.WriteString("Dưới đây là ảnh đại diện của từng cảnh trong một bộ phim, THEO ĐÚNG THỨ TỰ đính kèm. " +
		"Viết lời dẫn TIẾNG VIỆT cho từng cảnh, mỗi cảnh 1-2 câu nói vừa miệng trong khoảng thời lượng của cảnh " +
		"(người đọc khoảng 15 chữ mỗi 4 giây). Lời các cảnh phải NỐI MẠCH với nhau thành một bài kể liền.\n\n")
	var paths []string
	for _, i := range idx {
		s := m.Scenes[i]
		fmt.Fprintf(&b, "Cảnh %d: từ giây %.0f đến %.0f (dài %.0f giây) — ảnh đính kèm thứ %d.\n",
			s.Index, s.Start, s.End, s.End-s.Start, len(paths)+1)
		paths = append(paths, filepath.Join(st.DataDir, filepath.FromSlash(s.Frame)))
	}
	b.WriteString("\nTrả về DUY NHẤT một mảng JSON, không lời dẫn ngoài JSON: " +
		`[{"index": <số cảnh>, "text": "<lời dẫn>"}]`)

	out, err := cli.GenerateWithFiles(ctx, b.String(), paths)
	if err != nil {
		return fmt.Errorf("AI viết lời thất bại: %w", err)
	}
	items, err := parseNarration(out)
	if err != nil {
		return err
	}
	byIdx := map[int]string{}
	for _, it := range items {
		byIdx[it.Index] = strings.TrimSpace(it.Text)
	}
	for _, i := range idx {
		if t := byIdx[m.Scenes[i].Index]; t != "" {
			m.Scenes[i].Text = t
		}
	}
	return nil
}

type narrationItem struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
}

// parseNarration đọc JSON từ câu trả lời của model, chịu được rào ```json và
// chữ thừa quanh mảng — model hay "lịch sự" thêm một câu dẫn dù đã cấm.
func parseNarration(out string) ([]narrationItem, error) {
	s := strings.TrimSpace(out)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	if i := strings.Index(s, "["); i >= 0 {
		if j := strings.LastIndex(s, "]"); j > i {
			s = s[i : j+1]
		}
	}
	var items []narrationItem
	if err := json.Unmarshal([]byte(s), &items); err != nil {
		return nil, fmt.Errorf("AI trả về không đúng định dạng JSON: %w — nội dung: %.200s", err, out)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("AI không trả về lời dẫn nào")
	}
	return items, nil
}
