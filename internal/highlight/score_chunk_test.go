package highlight

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"bizstudio/internal/store"
)

// fakeLLM thay chỗ gọi AI thật. fn nhận prompt, trả chuỗi AI "đáp".
func fakeLLM(t *testing.T, fn func(prompt string) (string, error)) {
	t.Helper()
	old := runLLMFn
	runLLMFn = func(_ context.Context, _ *store.Store, prompt string) (string, error) {
		return fn(prompt)
	}
	t.Cleanup(func() { runLLMFn = old })
}

func candidates(n int) []Candidate {
	cs := make([]Candidate, n)
	for i := range cs {
		cs[i] = Candidate{
			Index: i,
			Start: float64(i) * 10, End: float64(i)*10 + 8,
			Text: fmt.Sprintf("câu số %d", i),
		}
	}
	return cs
}

// indexesIn đọc các số thứ tự [n] mà prompt đang hỏi.
func indexesIn(prompt string) []int {
	var out []int
	for _, line := range strings.Split(prompt, "\n") {
		if !strings.HasPrefix(line, "[") {
			continue
		}
		var i int
		if _, err := fmt.Sscanf(line, "[%d]", &i); err == nil {
			out = append(out, i)
		}
	}
	return out
}

func answer(idx []int) string {
	var b strings.Builder
	b.WriteString("[")
	for k, i := range idx {
		if k > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"i":%d,"diem":8,"vi":"hay"}`, i)
	}
	b.WriteString("]")
	return b.String()
}

// Không được gửi cả nghìn đoạn trong một lượt.
//
// Đo trên bản bóc băng thật: video 2 tiếng ra 1.107 đoạn. Bắt AI trả 1.107 dòng
// JSON một lượt thì hoặc bị cắt giữa chừng (mất trắng lượt chạy), hoặc nó chấm
// vài trăm dòng rồi tự đóng mảng — kiểu thứ hai hỏng LẶNG LẼ.
func TestScoreSplitsIntoChunks(t *testing.T) {
	const n = 250
	var sizes []int
	fakeLLM(t, func(p string) (string, error) {
		idx := indexesIn(p)
		sizes = append(sizes, len(idx))
		return answer(idx), nil
	})

	out, rep, err := Score(context.Background(), nil, candidates(n), 60, "", "auto", nil)
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if rep.Chunks < 2 {
		t.Fatalf("chỉ chia %d lô cho %d đoạn — vẫn gửi một cục", rep.Chunks, n)
	}
	for _, s := range sizes {
		if s > scoreChunk {
			t.Errorf("một lượt hỏi %d đoạn, vượt trần %d", s, scoreChunk)
		}
	}
	if rep.Scored != n {
		t.Errorf("chấm được %d/%d đoạn", rep.Scored, n)
	}
	if rep.Warn != "" {
		t.Errorf("không nên có cảnh báo khi chấm đủ, có: %q", rep.Warn)
	}
	for _, c := range out {
		if c.Score == 0 {
			t.Fatalf("đoạn %d không có điểm", c.Index)
		}
	}
}

// AI trả thiếu thì phải hỏi lại ĐÚNG những đoạn thiếu, không chấm lại cả lô.
func TestScoreRetriesOnlyMissing(t *testing.T) {
	first := true
	var retryAsked []int
	fakeLLM(t, func(p string) (string, error) {
		idx := indexesIn(p)
		if first {
			first = false
			return answer(idx[:len(idx)/2]), nil // cố tình trả một nửa
		}
		if retryAsked == nil {
			retryAsked = idx
		}
		return answer(idx), nil
	})

	_, rep, err := Score(context.Background(), nil, candidates(40), 60, "", "auto", nil)
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if rep.Retries != 1 {
		t.Errorf("Retries = %d, muốn 1", rep.Retries)
	}
	if len(retryAsked) != 20 {
		t.Errorf("hỏi lại %d đoạn, muốn đúng 20 đoạn còn thiếu — hỏi lại cả lô là phí tiền và phí thời gian",
			len(retryAsked))
	}
	if rep.Scored != 40 {
		t.Errorf("chấm được %d/40 đoạn sau khi hỏi lại", rep.Scored)
	}
}

// Đây là lỗi đã đo được trước khi sửa: AI chấm 1/3 đầu rồi đóng mảng đúng cú
// pháp, mã cũ nhận và dựng clip chỉ từ 2% đầu video, KHÔNG một dòng cảnh báo.
// Nay phải báo lỗi.
func TestScoreFailsWhenTooManyUnscored(t *testing.T) {
	fakeLLM(t, func(p string) (string, error) {
		idx := indexesIn(p)
		return answer(idx[:len(idx)/3]), nil // lô nào cũng thiếu, kể cả lần hỏi lại
	})

	_, rep, err := Score(context.Background(), nil, candidates(120), 60, "", "auto", nil)
	if err == nil {
		t.Fatal("chấm thiếu quá nửa mà vẫn báo thành công — đúng cái lỗi cần chặn")
	}
	if !strings.Contains(err.Error(), "không đáng tin") {
		t.Errorf("thông báo lỗi phải nói rõ kết quả không đáng tin, đang là: %v", err)
	}
	t.Logf("chặn đúng: chấm được %d/%d → %v", rep.Scored, rep.Total, err)
}

// Thiếu ít thì vẫn chạy, nhưng phải NÓI RA. Im lặng mới là vấn đề.
func TestScoreWarnsWhenSlightlyIncomplete(t *testing.T) {
	fakeLLM(t, func(p string) (string, error) {
		idx := indexesIn(p)
		return answer(idx[:len(idx)-1]), nil // lô nào cũng thiếu đúng 1 đoạn
	})

	_, rep, err := Score(context.Background(), nil, candidates(100), 60, "", "auto", nil)
	if err != nil {
		t.Fatalf("thiếu chút xíu thì không nên chết: %v", err)
	}
	if rep.Warn == "" {
		t.Fatal("thiếu đoạn mà không cảnh báo — người dùng tưởng clip đã xét cả video")
	}
	t.Logf("cảnh báo: %s", rep.Warn)
}

// AI bị cắt giữa chừng (JSON không đóng mảng) → phải báo lỗi rõ, không im.
func TestScoreFailsOnTruncatedJSON(t *testing.T) {
	fakeLLM(t, func(string) (string, error) {
		return `[{"i":0,"diem":9,"vi":"hay"},{"i":1,"diem":8,"vi":"tố`, nil
	})
	if _, _, err := Score(context.Background(), nil, candidates(30), 60, "", "auto", nil); err == nil {
		t.Fatal("JSON cụt mà vẫn báo thành công")
	}
}

// Mỗi lô phải nói rõ cần đủ bao nhiêu phần tử — chỉ nhắc "chấm cho tất cả" thì
// model dễ tự ý dừng sớm.
func TestPromptStatesExpectedCount(t *testing.T) {
	p := buildScorePrompt(candidates(37), 60, "", FindGenre("auto"))
	if !strings.Contains(p, "ĐỦ 37 phần tử") {
		t.Error("prompt không nêu rõ số phần tử phải trả về")
	}
}
