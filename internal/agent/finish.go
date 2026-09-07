package agent

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"bizstudio/internal/store"
)

// A result event is only a claim. Confirm process success and a fresh, valid
// artifact before publishing done, while still holding the project lease.
func (r *Runner) finishRun(ctx context.Context, sessionID, projectID string, result *streamEvent, waitErr error, stderr *bytes.Buffer, before outputSnapshot) {
	sess, ok := r.st.Session(sessionID)
	if !ok {
		return
	}
	if sess.Status == "stopped" {
		r.resetProjectAfterStop(projectID)
		return
	}
	var failure error
	switch {
	case ctx.Err() != nil:
		failure = fmt.Errorf("phiên AI đã hết thời gian hoặc bị hủy: %w", ctx.Err())
	case waitErr != nil:
		failure = fmt.Errorf("Claude kết thúc lỗi: %w", waitErr)
	case result == nil:
		failure = fmt.Errorf("Claude kết thúc mà không trả kết quả")
	case result.IsError || result.Subtype != "success":
		failure = fmt.Errorf("Claude chưa hoàn thành (%s)", result.Subtype)
	}
	if failure != nil {
		msg := failure.Error()
		if stderr != nil && strings.TrimSpace(stderr.String()) != "" {
			msg += " — " + truncate(stderr.String(), 500)
		}
		r.failSession(sessionID, projectID, msg)
		return
	}
	verifyCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	out, err := r.verifiedOutput(verifyCtx, projectID, before)
	if err != nil {
		r.failSession(sessionID, projectID, "Chưa thể hoàn thành video: "+err.Error()+". Có thể tiếp tục phiên để sửa kết quả.")
		return
	}
	// Stop may arrive during a long media validation.
	r.mu.Lock()
	defer r.mu.Unlock()
	sess, ok = r.st.Session(sessionID)
	if !ok {
		return
	}
	if sess.Status == "stopped" {
		r.resetProjectAfterStop(projectID)
		return
	}
	if ctx.Err() != nil {
		r.failSessionLocked(sessionID, projectID, "Phiên AI đã hết thời gian trước khi hoàn tất")
		return
	}
	p, ok := r.st.Project(projectID)
	if !ok {
		r.failSessionLocked(sessionID, projectID, "Dự án không còn tồn tại")
		return
	}
	p.OutputFile, p.Status, p.Progress = out, "done", 6
	r.st.SaveProject(&p)
	r.broadcast("project", p)
	sess.Status, sess.EndedAt = "done", time.Now()
	r.st.SaveSession(&sess)
	r.broadcast("session", sess)
}

func (r *Runner) failSession(sessionID, projectID, msg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failSessionLocked(sessionID, projectID, msg)
}

func (r *Runner) failSessionLocked(sessionID, projectID, msg string) {
	r.addEvent(&store.SessionEvent{SessionID: sessionID, Type: "error", Payload: msg})
	r.st.AddLog("error", "agent", msg)
	if sess, ok := r.st.Session(sessionID); ok {
		if sess.Status == "stopped" {
			r.resetProjectAfterStop(projectID)
			return
		}
		sess.Status, sess.EndedAt = "error", time.Now()
		r.st.SaveSession(&sess)
		r.broadcast("session", sess)
	}
	if p, ok := r.st.Project(projectID); ok {
		p.Status = "error"
		r.st.SaveProject(&p)
		r.broadcast("project", p)
	}
}

func (r *Runner) resetProjectAfterStop(projectID string) {
	p, ok := r.st.Project(projectID)
	if !ok || p.Status != "running" {
		return
	}
	p.Status = "draft"
	r.st.SaveProject(&p)
	r.broadcast("project", p)
}
