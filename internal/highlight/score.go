package highlight

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"bizstudio/internal/gemini"
	"bizstudio/internal/openaiapi"
	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

const scoreSystem = "Bạn là người dựng video ngắn tiếng Việt, chuyên chọn đoạn đắt nhất từ video dài. " +
	"Luôn trả về đúng JSON được yêu cầu, không thêm bất kỳ nội dung nào khác."

// Score nhờ AI chấm điểm từng đoạn ứng viên.
//
// Cố ý dùng AI chứ không dùng công thức: "đoạn hấp dẫn" là chuyện nội dung —
// một câu nói to, nói nhanh, hay có nhiều từ chưa chắc đã đáng giữ, mà một câu
// nói khẽ đúng chỗ thì đáng. Công thức chỉ đo được âm lượng và mật độ chữ, hai
// thứ gần như không liên quan tới việc người xem có muốn xem tiếp hay không.
//
// Không có khoá AI thì trả lỗi rõ ràng chứ KHÔNG lặng lẽ rơi về chấm bừa: cắt
// nhầm đoạn còn tệ hơn không cắt, vì người dùng tưởng máy đã chọn hộ.
func Score(ctx context.Context, st *store.Store, cs []Candidate, targetSec int, goal string) ([]Candidate, error) {
	if len(cs) == 0 {
		return nil, fmt.Errorf("không có đoạn nào để chấm — bản bóc băng rỗng")
	}
	raw, err := runLLM(ctx, st, buildScorePrompt(cs, targetSec, goal))
	if err != nil {
		return nil, err
	}
	marks := parseScores(raw)
	if len(marks) == 0 {
		return nil, fmt.Errorf("AI không trả về điểm nào — thử lại hoặc đổi engine (nhận được: %s)", shortText(raw, 200))
	}
	out := append([]Candidate(nil), cs...)
	n := 0
	for i := range out {
		m, ok := marks[out[i].Index]
		if !ok {
			continue
		}
		out[i].Score, out[i].Why = m.Score, strings.TrimSpace(m.Why)
		n++
	}
	if n == 0 {
		return nil, fmt.Errorf("AI chấm điểm cho những số thứ tự không có thật — thử lại")
	}
	return out, nil
}

func buildScorePrompt(cs []Candidate, targetSec int, goal string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `Dưới đây là bản bóc băng một video dài, đã chia thành các đoạn có đánh số và mốc thời gian.
Nhiệm vụ: chấm điểm mỗi đoạn theo mức ĐÁNG GIỮ khi rút video này thành clip ngắn khoảng %d giây.

Thang điểm 0 đến 10:
- 9-10: mở được clip — câu gây tò mò, con số bất ngờ, tuyên bố mạnh, cao trào.
- 6-8: có thông tin thật, cụ thể, đứng một mình vẫn hiểu.
- 3-5: cần ngữ cảnh mới hiểu, hoặc chỉ là câu nối.
- 0-2: chào hỏi, lặp lại, ê a, lạc đề, nói hụt.

Nguyên tắc:
- Chấm theo NỘI DUNG, không theo độ dài. Đoạn ngắn mà đắt vẫn điểm cao.
- Đoạn đứng một mình người xem không hiểu thì hạ điểm, dù nội dung hay — clip ngắn không có chỗ giải thích.
- Không bịa nội dung không có trong bản bóc băng.
`, targetSec)
	if g := strings.TrimSpace(goal); g != "" {
		fmt.Fprintf(&b, "- Người dùng muốn clip nhắm vào: %s. Đoạn phục vụ ý này thì cộng điểm.\n", g)
	}
	b.WriteString("\nCác đoạn:\n")
	for _, c := range cs {
		fmt.Fprintf(&b, "[%d] %.1fs-%.1fs (%.1fs): %s\n", c.Index, c.Start, c.End, c.Dur(), oneLine(c.Text))
	}
	b.WriteString(`
Trả về DUY NHẤT một JSON mảng, mỗi phần tử là {"i": số thứ tự đoạn, "diem": số 0-10, "vi": "lý do ngắn gọn dưới 12 từ"}:
[{"i":0,"diem":8,"vi":"mở bằng con số gây sốc"}]
Chấm cho TẤT CẢ các đoạn. Không giải thích, không markdown, không văn bản nào khác ngoài JSON.`)
	return b.String()
}

type mark struct {
	Score float64
	Why   string
}

var reFence = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")

// parseScores đọc kết quả AI. Chịu được vài kiểu viết lệch hay gặp: bọc trong
// khối mã, đặt tên khoá bằng tiếng Anh, điểm trả về dạng chuỗi.
func parseScores(raw string) map[int]mark {
	s := strings.TrimSpace(raw)
	if m := reFence.FindStringSubmatch(s); m != nil {
		s = m[1]
	}
	if i := strings.Index(s, "["); i >= 0 {
		if j := strings.LastIndex(s, "]"); j > i {
			s = s[i : j+1]
		}
	}
	var rows []map[string]any
	if json.Unmarshal([]byte(s), &rows) != nil {
		return nil
	}
	out := map[int]mark{}
	for _, r := range rows {
		idx, ok := pickInt(r, "i", "index", "idx", "stt")
		if !ok {
			continue
		}
		sc, ok := pickFloat(r, "diem", "score", "diem_so", "point")
		if !ok {
			continue
		}
		if sc < 0 {
			sc = 0
		}
		if sc > 10 {
			sc = 10
		}
		out[idx] = mark{Score: sc, Why: pickStr(r, "vi", "why", "ly_do", "reason")}
	}
	return out
}

func pickInt(m map[string]any, keys ...string) (int, bool) {
	if f, ok := pickFloat(m, keys...); ok {
		return int(f), true
	}
	return 0, false
}

func pickFloat(m map[string]any, keys ...string) (float64, bool) {
	for _, k := range keys {
		switch v := m[k].(type) {
		case float64:
			return v, true
		case int:
			return float64(v), true
		case string:
			var f float64
			if _, err := fmt.Sscanf(strings.TrimSpace(v), "%g", &f); err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

func pickStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

func shortText(s string, n int) string {
	s = oneLine(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ---------- gọi engine ----------

// runLLM gọi engine theo Cấu hình & API, thứ tự ưu tiên giống các module khác:
// API Trực Tiếp (OpenAI-compatible) → Gemini → Claude CLI.
func runLLM(ctx context.Context, st *store.Store, prompt string) (string, error) {
	set := st.Settings()
	switch {
	case strings.TrimSpace(set.OpenAIKey) != "":
		return openaiapi.NewFromSettings(st).ChatText(ctx, scoreSystem, prompt)
	case strings.TrimSpace(set.GeminiAPIKey) != "":
		return gemini.NewFromSettings(st).GenerateText(ctx, scoreSystem, prompt)
	default:
		return runClaude(ctx, set.ClaudeBin, set.ClaudeModel, scoreSystem+"\n\n"+prompt)
	}
}

func runClaude(ctx context.Context, bin, model, prompt string) (string, error) {
	if strings.TrimSpace(bin) == "" {
		bin = "claude"
	}
	args := []string{"-p", "--output-format", "text"}
	if m := strings.TrimSpace(model); m != "" {
		args = append(args, "--model", m)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = strings.NewReader(prompt)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("chạy Claude CLI (%s) thất bại: %w — %s",
			bin, err, util.ClaudeFailReason(so.String(), se.String()))
	}
	out := strings.TrimSpace(so.String())
	if out == "" {
		return "", fmt.Errorf("Claude CLI không trả về nội dung nào — %s", util.ClaudeFailReason("", se.String()))
	}
	return out, nil
}

// Pick chọn các đoạn điểm cao cho vừa targetSec, rồi TRẢ VỀ THEO ĐÚNG THỨ TỰ
// THỜI GIAN của video gốc.
//
// Sắp lại theo thời gian là bắt buộc, không phải để cho đẹp: ghép các đoạn theo
// thứ tự điểm thì câu chuyện nhảy cóc tới lui, người xem không lần ra mạch. Chọn
// theo điểm, xếp theo thời gian — hai việc khác nhau.
func Pick(cs []Candidate, targetSec int, minScore float64) []Candidate {
	if targetSec <= 0 || len(cs) == 0 {
		return nil
	}
	ranked := append([]Candidate(nil), cs...)
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		return ranked[i].Start < ranked[j].Start // hoà điểm thì lấy đoạn sớm hơn
	})

	var chosen []Candidate
	var total float64
	for _, c := range ranked {
		if c.Score < minScore {
			break // đã sắp giảm dần, dưới ngưỡng thì phần còn lại cũng dưới
		}
		if total+c.Dur() > float64(targetSec) {
			continue // đoạn này quá dài cho chỗ còn lại, thử đoạn kém hơn mà ngắn hơn
		}
		chosen = append(chosen, c)
		total += c.Dur()
	}
	sort.Slice(chosen, func(i, j int) bool { return chosen[i].Start < chosen[j].Start })
	return chosen
}
