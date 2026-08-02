package agent

import (
	"encoding/json"
	"strings"
	"time"

	"bizstudio/internal/store"
)

// streamEvent — một dòng NDJSON của claude --output-format stream-json.
type streamEvent struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype"`
	SessionID string          `json:"session_id"`
	Model     string          `json:"model"`
	Message   *streamMessage  `json:"message"`
	Result    json.RawMessage `json:"result"`
	NumTurns  int             `json:"num_turns"`
	CostUSD   float64         `json:"total_cost_usd"`
}

type streamMessage struct {
	Content json.RawMessage `json:"content"`
}

type streamBlock struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input"`
	Content json.RawMessage `json:"content"`
}

// blocks giải mã message.content thành mảng block (an toàn khi content là string).
func (ev streamEvent) blocks() []streamBlock {
	if ev.Message == nil || len(ev.Message.Content) == 0 {
		return nil
	}
	var out []streamBlock
	if err := json.Unmarshal(ev.Message.Content, &out); err != nil {
		return nil
	}
	return out
}

// resultText trả field "result" dạng chuỗi (an toàn khi không phải string).
func (ev streamEvent) resultText() string {
	if len(ev.Result) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(ev.Result, &s); err == nil {
		return s
	}
	return string(ev.Result)
}

// handleLine xử lý một dòng stream-json. Trả true nếu là event "result".
func (r *Runner) handleLine(sessionID, projectID string, line []byte) bool {
	var ev streamEvent
	if err := json.Unmarshal(line, &ev); err != nil {
		r.st.AddLog("warn", "agent", "dòng stream không hợp lệ: "+truncate(string(line), 200))
		return false
	}
	switch ev.Type {
	case "system":
		if ev.Subtype == "init" {
			r.onInit(sessionID, ev)
		}
	case "assistant":
		r.onAssistant(sessionID, ev)
	case "user":
		r.onToolResult(sessionID, ev)
	case "result":
		r.onResult(sessionID, projectID, ev)
		return true
	}
	return false
}

// onInit lưu Claude session ID + model của phiên.
func (r *Runner) onInit(sessionID string, ev streamEvent) {
	sess, ok := r.st.Session(sessionID)
	if !ok {
		return
	}
	sess.ClaudeSessionID = ev.SessionID
	r.st.SaveSession(&sess)
	r.broadcast("session", sess)

	payload, _ := json.Marshal(map[string]string{"model": ev.Model})
	r.addEvent(&store.SessionEvent{SessionID: sessionID, Type: "init", Payload: string(payload)})
}

// onAssistant xử lý các block text / tool_use của assistant.
func (r *Runner) onAssistant(sessionID string, ev streamEvent) {
	for _, b := range ev.blocks() {
		switch b.Type {
		case "text":
			if strings.TrimSpace(b.Text) == "" {
				continue
			}
			r.addEvent(&store.SessionEvent{SessionID: sessionID, Type: "text", Payload: b.Text})
		case "tool_use":
			r.addEvent(&store.SessionEvent{
				SessionID: sessionID,
				Type:      "tool",
				Name:      b.Name,
				Payload:   truncate(string(b.Input), 2000),
			})
		}
	}
}

// onToolResult xử lý các block tool_result trong message user.
func (r *Runner) onToolResult(sessionID string, ev streamEvent) {
	for _, b := range ev.blocks() {
		if b.Type != "tool_result" {
			continue
		}
		r.addEvent(&store.SessionEvent{
			SessionID: sessionID,
			Type:      "tool_result",
			Payload:   truncate(decodeContent(b.Content), 500),
		})
	}
}

// onResult chốt phiên: lưu kết quả, số lượt, chi phí, trạng thái + finalize dự án.
func (r *Runner) onResult(sessionID, projectID string, ev streamEvent) {
	r.addEvent(&store.SessionEvent{SessionID: sessionID, Type: "result", Payload: ev.resultText()})

	sess, ok := r.st.Session(sessionID)
	if !ok {
		return
	}
	sess.NumTurns = ev.NumTurns
	sess.CostUSD = ev.CostUSD
	if ev.Subtype == "success" {
		sess.Status = "done"
	} else {
		sess.Status = "error"
	}
	sess.EndedAt = time.Now()
	r.st.SaveSession(&sess)
	r.broadcast("session", sess)

	r.finalizeProject(projectID, sess.Status == "done")
}

// decodeContent lấy text từ content của tool_result (string hoặc mảng block).
func decodeContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var arr []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &arr); err == nil {
		var parts []string
		for _, it := range arr {
			if it.Text != "" {
				parts = append(parts, it.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return string(raw)
}

// truncate cắt chuỗi theo rune (an toàn UTF-8 tiếng Việt).
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n]) + "…"
}
