// Package tts — giọng đọc: liệt kê voice (macOS say + Gemini) và tổng hợp giọng nói.
package tts

import (
	"context"
	"sort"
	"strings"
	"time"

	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

// Voice — một giọng đọc khả dụng.
type Voice struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Gender string `json:"gender"`
	Lang   string `json:"lang"`
	Engine string `json:"engine"`
}

// geminiVoices — 5 giọng Gemini luôn có mặt trong danh sách.
var geminiVoices = []Voice{
	{ID: "Kore", Name: "Kore", Gender: "Nữ", Lang: "đa ngôn ngữ", Engine: "gemini"},
	{ID: "Puck", Name: "Puck", Gender: "Nam", Lang: "đa ngôn ngữ", Engine: "gemini"},
	{ID: "Charon", Name: "Charon", Gender: "Nam", Lang: "đa ngôn ngữ", Engine: "gemini"},
	{ID: "Fenrir", Name: "Fenrir", Gender: "Nam", Lang: "đa ngôn ngữ", Engine: "gemini"},
	{ID: "Aoede", Name: "Aoede", Gender: "Nữ", Lang: "đa ngôn ngữ", Engine: "gemini"},
}

// VoicesFor liệt kê giọng đọc: VieNeu-TTS (nếu đã cài — xếp ĐẦU, khuyên dùng)
// + macOS `say -v ?` + 5 giọng Gemini. Giọng vi xếp trước trong từng nhóm.
func VoicesFor(st *store.Store) []Voice {
	return append(vieneuVoices(st), Voices()...)
}

// Voices liệt kê giọng đọc: macOS `say -v ?` + 5 giọng Gemini. Giọng vi_VN xếp đầu.
func Voices() []Voice {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var out []Voice
	raw, err := util.Run(ctx, "say", "-v", "?")
	if err != nil {
		raw, err = util.Run(ctx, "say", "-v", "'?'")
	}
	if err == nil {
		out = append(out, parseSayVoices(raw)...)
	}
	out = append(out, geminiVoices...)

	sort.SliceStable(out, func(i, j int) bool { return voiceRank(out[i]) < voiceRank(out[j]) })
	return out
}

// voiceRank: 0 nếu giọng tiếng Việt (vi_VN), 1 nếu khác — để sort vi lên đầu.
func voiceRank(v Voice) int {
	if strings.HasPrefix(strings.ToLower(v.Lang), "vi") {
		return 0
	}
	return 1
}

// parseSayVoices phân tích output của `say -v ?`:
// mỗi dòng dạng "Tên (có thể có khoảng trắng)    vi_VN    # câu mẫu".
func parseSayVoices(raw string) []Voice {
	var out []Voice
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		left := line
		if i := strings.Index(line, "#"); i >= 0 {
			left = line[:i]
		}
		fields := strings.Fields(left)
		if len(fields) < 2 {
			continue
		}
		lang := fields[len(fields)-1]
		name := strings.Join(fields[:len(fields)-1], " ")
		out = append(out, Voice{ID: name, Name: name, Lang: lang, Engine: "say"})
	}
	return out
}
