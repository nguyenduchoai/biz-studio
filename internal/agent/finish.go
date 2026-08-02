package agent

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/store"
)

// finishRun xử lý khi tiến trình claude thoát. Nếu đã có event "result" thì
// onResult đã cập nhật đầy đủ; nếu không → phiên lỗi (hoặc giữ "stopped" nếu bị Stop).
func (r *Runner) finishRun(sessionID, projectID string, gotResult bool, waitErr error, stderr *bytes.Buffer) {
	if gotResult {
		return
	}
	sess, ok := r.st.Session(sessionID)
	if !ok {
		return
	}
	if sess.Status == "stopped" {
		if sess.EndedAt.IsZero() {
			sess.EndedAt = time.Now()
			r.st.SaveSession(&sess)
			r.broadcast("session", sess)
		}
		r.resetProjectAfterStop(projectID)
		return
	}

	msg := "claude CLI kết thúc mà không trả kết quả"
	if waitErr != nil {
		msg = "claude CLI lỗi: " + waitErr.Error()
	}
	if s := strings.TrimSpace(stderr.String()); s != "" {
		msg += " — " + truncate(s, 500)
	}
	r.addEvent(&store.SessionEvent{SessionID: sessionID, Type: "error", Payload: msg})
	r.st.AddLog("error", "agent", msg)

	sess.Status = "error"
	sess.EndedAt = time.Now()
	r.st.SaveSession(&sess)
	r.broadcast("session", sess)

	r.finalizeProject(projectID, false)
}

// failSession dùng khi không khởi động được tiến trình claude.
func (r *Runner) failSession(sessionID, projectID, msg string) {
	r.addEvent(&store.SessionEvent{SessionID: sessionID, Type: "error", Payload: msg})
	r.st.AddLog("error", "agent", msg)

	if sess, ok := r.st.Session(sessionID); ok {
		sess.Status = "error"
		sess.EndedAt = time.Now()
		r.st.SaveSession(&sess)
		r.broadcast("session", sess)
	}
	r.finalizeProject(projectID, false)
}

// resetProjectAfterStop trả dự án đang "running" về "draft" khi phiên bị dừng chủ động.
func (r *Runner) resetProjectAfterStop(projectID string) {
	p, ok := r.st.Project(projectID)
	if !ok || p.Status != "running" {
		return
	}
	p.Status = "draft"
	r.st.SaveProject(&p)
	r.broadcast("project", p)
}

// finalizeProject cập nhật dự án khi phiên kết thúc: done → tìm output; lỗi → error.
func (r *Runner) finalizeProject(projectID string, sessionDone bool) {
	p, ok := r.st.Project(projectID)
	if !ok {
		return
	}
	if !sessionDone {
		p.Status = "error"
		r.st.SaveProject(&p)
		r.broadcast("project", p)
		return
	}
	if out := r.findOutput(projectID); out != "" {
		p.OutputFile = out
	} else {
		r.st.AddLog("warn", "agent", "phiên hoàn tất nhưng không tìm thấy video output cho dự án "+projectID)
	}
	p.Status = "done"
	p.Progress = 6
	r.st.SaveProject(&p)
	r.broadcast("project", p)
}

// findOutput tìm video kết quả, trả đường dẫn tương đối dataDir
// ("projects/<id>/outputs/<file>") hoặc "" nếu không có.
// Ưu tiên meta.json {"status","output"}, sau đó file .mp4 mới nhất trong outputs/.
func (r *Runner) findOutput(projectID string) string {
	dir := filepath.Join(r.dataDir, "projects", projectID)

	if raw, err := os.ReadFile(filepath.Join(dir, "meta.json")); err == nil {
		var meta struct {
			Status string `json:"status"`
			Output string `json:"output"`
		}
		if json.Unmarshal(raw, &meta) == nil && meta.Output != "" {
			rel := path.Clean(filepath.ToSlash(meta.Output))
			if rel != "." && !strings.HasPrefix(rel, "..") && !path.IsAbs(rel) {
				if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err == nil {
					return path.Join("projects", projectID, rel)
				}
			}
		}
	}

	entries, err := os.ReadDir(filepath.Join(dir, "outputs"))
	if err != nil {
		return ""
	}
	var newest string
	var newestT time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".mp4") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if newest == "" || info.ModTime().After(newestT) {
			newest, newestT = e.Name(), info.ModTime()
		}
	}
	if newest == "" {
		return ""
	}
	return path.Join("projects", projectID, "outputs", newest)
}
