package tts

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"bizstudio/internal/store"
)

// ClonePrefix — tiền tố voiceID của giọng nhân bản: "clone:<cloneID>"
// (có thể kèm phong cách: "clone:<cloneID>@tin_tuc").
const ClonePrefix = "clone:"

// cloneRelDir — thư mục chứa clip mẫu, tương đối DataDir (dùng "/" như store.CloneVoice.Path).
const cloneRelDir = "vieneu/clones"

// IsCloneVoice — voiceID có trỏ tới giọng nhân bản hay không.
func IsCloneVoice(voiceID string) bool {
	return strings.HasPrefix(strings.TrimSpace(voiceID), ClonePrefix)
}

// CloneVoiceID dựng voiceID chuẩn cho một giọng nhân bản.
func CloneVoiceID(cloneID string) string { return ClonePrefix + cloneID }

// CloneRelPath — đường dẫn clip mẫu tương đối DataDir (lưu vào store.CloneVoice.Path).
func CloneRelPath(cloneID string) string { return path.Join(cloneRelDir, cloneID+".wav") }

// CloneDir trả thư mục tuyệt đối chứa clip mẫu, tự tạo nếu chưa có.
func CloneDir(st *store.Store) (string, error) {
	dir := filepath.Join(st.DataDir, filepath.FromSlash(cloneRelDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("không tạo được thư mục giọng nhân bản %s: %w", dir, err)
	}
	return dir, nil
}

// cloneVoices liệt kê giọng nhân bản dưới dạng Voice (engine "clone").
func cloneVoices(st *store.Store) []Voice {
	list := st.CloneVoices()
	out := make([]Voice, 0, len(list))
	for _, c := range list {
		name := strings.TrimSpace(c.Name)
		if name == "" {
			name = "Giọng nhân bản"
		}
		out = append(out, Voice{
			ID:     CloneVoiceID(c.ID),
			Name:   name,
			Gender: c.Gender,
			Lang:   "vi-VN · nhân bản",
			Engine: "clone",
		})
	}
	return out
}

// cloneRefFor trả đường dẫn tuyệt đối clip mẫu khi voiceID là giọng nhân bản.
// voiceID thường ("Minh Đức", rỗng…) → trả "" và không lỗi.
func cloneRefFor(st *store.Store, voiceID string) (string, error) {
	if !IsCloneVoice(voiceID) {
		return "", nil
	}
	id := strings.TrimPrefix(strings.TrimSpace(voiceID), ClonePrefix)
	if i := strings.LastIndex(id, "@"); i >= 0 { // bỏ hậu tố @phong_cách
		id = id[:i]
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("voiceID giọng nhân bản thiếu mã (dạng đúng: clone:<id>)")
	}
	c, ok := st.CloneVoice(id)
	if !ok {
		return "", fmt.Errorf("không tìm thấy giọng nhân bản %q — có thể đã bị xoá, hãy chọn giọng khác", id)
	}
	ref := filepath.Join(st.DataDir, filepath.FromSlash(c.Path))
	if _, err := os.Stat(ref); err != nil {
		return "", fmt.Errorf("thiếu clip mẫu của giọng %q (%s) — hãy tạo lại giọng nhân bản", c.Name, c.Path)
	}
	return ref, nil
}
