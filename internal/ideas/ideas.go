// Package ideas — Ý tưởng & Hàng đợi sản xuất.
//
// Hai phần:
//   - Generate: nhờ LLM đề xuất hàng loạt ý tưởng video cho một chủ đề / kênh,
//     người dùng duyệt rồi mới đưa vào hàng đợi.
//   - Runner: hàng đợi sản xuất chạy TUẦN TỰ, mỗi lúc đúng MỘT ý tưởng (viết
//     kịch bản → giọng đọc → storyboard → dựng video). Chạy song song sẽ tranh
//     nhau Chrome / ffmpeg / TTS nên tuyệt đối không được nới thành nhiều luồng.
package ideas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"bizstudio/internal/store"
)

const (
	// DefaultCount — số ý tưởng mặc định mỗi lần sinh.
	DefaultCount = 8
	// MaxCount — trần số ý tưởng mỗi lần sinh (nhiều hơn thì AI trả lan man, trùng ý).
	MaxCount = 20

	// maxTopicRunes — cắt bớt mô tả chủ đề quá dài trước khi gửi LLM.
	maxTopicRunes = 8000
	// maxTitleRunes / maxFieldRunes — giới hạn hiển thị của từng trường ý tưởng.
	maxTitleRunes = 160
	maxFieldRunes = 400
	// maxKeywords — số từ khóa giữ lại cho mỗi ý tưởng.
	maxKeywords = 8
)

// Generate nhờ LLM đề xuất count ý tưởng video cho topic (chủ đề hoặc mô tả
// kênh). audience / tone là gợi ý thêm, để trống cũng được.
//
// Ý tưởng trả về ở trạng thái "proposed" và CHƯA được lưu store — người gọi tự
// gán cấu hình khung hình rồi lưu.
func Generate(ctx context.Context, st *store.Store, topic string, count int, audience, tone string) ([]store.Idea, error) {
	t := strings.TrimSpace(topic)
	if t == "" {
		return nil, errors.New("chưa có chủ đề — nhập chủ đề hoặc mô tả kênh trước khi sinh ý tưởng")
	}
	if r := []rune(t); len(r) > maxTopicRunes {
		t = string(r[:maxTopicRunes])
	}
	count = ClampCount(count)

	raw, err := runLLM(ctx, st, buildIdeaPrompt(t, count, audience, tone))
	if err != nil {
		return nil, err
	}
	list := parseIdeas(raw, count)
	if len(list) == 0 {
		return nil, fmt.Errorf("AI không trả về ý tưởng nào — thử lại hoặc đổi engine trong \"Cấu hình & API\" (nội dung nhận được: %s)",
			shortText(raw, 200))
	}
	return list, nil
}

// ClampCount ép số lượng ý tưởng về khoảng hợp lệ (1..MaxCount).
func ClampCount(n int) int {
	switch {
	case n <= 0:
		return DefaultCount
	case n > MaxCount:
		return MaxCount
	default:
		return n
	}
}

// buildIdeaPrompt dựng prompt tiếng Việt yêu cầu LLM trả JSON mảng ý tưởng.
func buildIdeaPrompt(topic string, count int, audience, tone string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `Đề xuất %d ý tưởng video ngắn cho chủ đề (hoặc kênh) bên dưới.

Quy tắc bắt buộc:
- Tiêu đề tiếng Việt, cụ thể và hấp dẫn, nói rõ người xem nhận được gì. KHÔNG giật tít rẻ tiền, không hứa hẹn quá đà, không emoji, không markdown, không dấu ngoặc kép.
- Mỗi ý tưởng một góc tiếp cận KHÁC nhau, không trùng ý, không nói lại cùng một nội dung.
- "angle": 1 đến 2 câu tóm tắt nội dung và góc tiếp cận của video.
- "hook": đúng 1 câu mở đầu giữ chân người xem trong 3 giây đầu.
- "keywords": 3 đến 6 từ khóa ngắn bằng tiếng Việt.
`, count)
	if a := strings.TrimSpace(audience); a != "" {
		fmt.Fprintf(&b, "- Đối tượng xem: %s — chọn ý tưởng và cách nói phù hợp với nhóm này.\n", shortText(a, 300))
	}
	if t := strings.TrimSpace(tone); t != "" {
		fmt.Fprintf(&b, "- Tông giọng: %s.\n", shortText(t, 200))
	}
	fmt.Fprintf(&b, `
Trả về DUY NHẤT một JSON mảng gồm ĐÚNG %d phần tử, mỗi phần tử dạng:
{"title":"…","angle":"…","hook":"…","keywords":["…","…"]}
Không giải thích, không markdown, không văn bản nào khác ngoài JSON.

Chủ đề / kênh:
%s`, count, topic)
	return b.String()
}

// rawIdea — một ý tưởng LLM trả về; Keywords để dạng thô vì model hay trả chuỗi
// "a, b, c" thay vì mảng.
type rawIdea struct {
	Title    string          `json:"title"`
	Angle    string          `json:"angle"`
	Hook     string          `json:"hook"`
	Keywords json.RawMessage `json:"keywords"`
}

// parseIdeas đọc kết quả LLM thành danh sách ý tưởng: ưu tiên JSON mảng object,
// không parse được thì coi mỗi dòng là một tiêu đề.
func parseIdeas(raw string, limit int) []store.Idea {
	body := stripFence(raw)
	items := parseJSONIdeas(body)
	if len(items) == 0 {
		items = parseLineIdeas(body)
	}
	out := make([]store.Idea, 0, len(items))
	for _, it := range items {
		title := shortText(cleanLine(it.Title), maxTitleRunes)
		if title == "" {
			continue
		}
		out = append(out, store.Idea{
			Title:    title,
			Angle:    shortText(cleanLine(it.Angle), maxFieldRunes),
			Hook:     shortText(cleanLine(it.Hook), maxFieldRunes),
			Keywords: decodeKeywords(it.Keywords),
			Status:   "proposed",
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

// parseJSONIdeas thử đọc mảng JSON object; mảng chuỗi cũng chấp nhận (chỉ có tiêu đề).
func parseJSONIdeas(body string) []rawIdea {
	arr := extractJSONArray(body)
	if arr == "" {
		return nil
	}
	var objs []rawIdea
	if err := json.Unmarshal([]byte(arr), &objs); err == nil && len(objs) > 0 {
		return objs
	}
	var titles []string
	if err := json.Unmarshal([]byte(arr), &titles); err == nil {
		out := make([]rawIdea, 0, len(titles))
		for _, t := range titles {
			out = append(out, rawIdea{Title: t})
		}
		return out
	}
	return nil
}

// parseLineIdeas dự phòng khi LLM không trả JSON: mỗi dòng là một tiêu đề.
func parseLineIdeas(body string) []rawIdea {
	out := make([]rawIdea, 0, 16)
	for _, ln := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if t := cleanLine(ln); t != "" {
			out = append(out, rawIdea{Title: t})
		}
	}
	return out
}

// decodeKeywords đọc từ khóa ở cả hai dạng model hay trả: mảng chuỗi hoặc một
// chuỗi ngăn cách bằng dấu phẩy.
func decodeKeywords(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		var joined string
		if err := json.Unmarshal(raw, &joined); err != nil {
			return []string{}
		}
		list = strings.Split(joined, ",")
	}
	return NormalizeKeywords(list)
}

// NormalizeKeywords làm sạch danh sách từ khóa: bỏ rỗng, bỏ trùng, giới hạn số lượng.
func NormalizeKeywords(list []string) []string {
	out := make([]string, 0, len(list))
	seen := make(map[string]bool, len(list))
	for _, k := range list {
		k = cleanLine(k)
		if k == "" || seen[strings.ToLower(k)] {
			continue
		}
		seen[strings.ToLower(k)] = true
		out = append(out, shortText(k, 60))
		if len(out) >= maxKeywords {
			break
		}
	}
	return out
}

// SourceText dựng nội dung nguồn cho phiên Text → Video từ một ý tưởng: tiêu đề
// + góc tiếp cận + hook + từ khóa, để AI viết kịch bản bám đúng ý đã duyệt.
func SourceText(idea store.Idea) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Chủ đề video: %s\n", strings.TrimSpace(idea.Title))
	if a := strings.TrimSpace(idea.Angle); a != "" {
		fmt.Fprintf(&b, "\nGóc tiếp cận: %s\n", a)
	}
	if h := strings.TrimSpace(idea.Hook); h != "" {
		fmt.Fprintf(&b, "\nCâu mở đầu giữ chân người xem: %s\n", h)
	}
	if len(idea.Keywords) > 0 {
		fmt.Fprintf(&b, "\nTừ khóa cần nhắc tới: %s\n", strings.Join(idea.Keywords, ", "))
	}
	b.WriteString("\nHãy viết kịch bản lời đọc cho video này, bám đúng chủ đề và góc tiếp cận ở trên.")
	return strings.TrimSpace(b.String())
}

// ValidStatus — các trạng thái hợp lệ của một ý tưởng.
func ValidStatus(s string) bool {
	switch s {
	case "proposed", "approved", "rejected", "queued", "producing", "done", "error":
		return true
	}
	return false
}

// CanQueue — chỉ ý tưởng chưa/không còn đang sản xuất mới được xếp hàng đợi.
func CanQueue(status string) bool {
	switch status {
	case "proposed", "approved", "rejected", "error", "":
		return true
	}
	return false
}
