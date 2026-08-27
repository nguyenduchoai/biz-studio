package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"path/filepath"
	"time"

	"bizstudio/internal/setup"
	"bizstudio/internal/util"
)

const (
	fullSetupID  = "full"
	fullSetupMax = 4 * time.Hour
	fullPlanTTL  = 5 * time.Minute
)

type fullSetupGrant struct {
	ToolIDs []string
	Expires time.Time
}

func (s *Server) handleSetupFullPlan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	statuses := s.setupStatuses(ctx)
	windows := setup.CheckWindowsReadiness(ctx)
	selected := make([]toolStatus, 0, len(statuses))
	needsLogin := false
	running := fullSetupPlanRunning(statuses)
	for _, status := range statuses {
		if status.Full && !status.Installed {
			selected = append(selected, status)
		}
		if status.ID == "claude" && status.NeedsLogin {
			needsLogin = true
		}
	}
	planID := ""
	if len(selected) > 0 {
		ids := make([]string, 0, len(selected))
		for _, status := range selected {
			ids = append(ids, status.ID)
		}
		planID = s.createFullSetupGrant(ids, time.Now())
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"planID":           planID,
		"tools":            selected,
		"statuses":         statuses,
		"needsLogin":       needsLogin,
		"needsSetup":       len(selected) > 0 || needsLogin || windows.NeedsPreparation,
		"running":          running,
		"windowsPreparing": setupIsRunning(windowsPrepareID),
		"windows":          windows,
		"note":             "Chỉ cài thành phần còn thiếu. VieNeu và Whisper có thể tải model lớn; Claude đăng nhập riêng sau khi cài.",
	})
}

func fullSetupPlanRunning(statuses []toolStatus) bool {
	if setupIsRunning(windowsPrepareID) {
		return true
	}
	for _, status := range statuses {
		if status.Running {
			return true
		}
	}
	return false
}

func (s *Server) handleSetupFullRun(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PlanID    string `json:"planID"`
		Confirmed bool   `json:"confirmed"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	if !body.Confirmed || body.PlanID == "" {
		httpErr(w, http.StatusBadRequest, "cần xác nhận kế hoạch cài đặt hiện tại")
		return
	}
	toolIDs, ok := s.consumeFullSetupGrant(body.PlanID, time.Now())
	if !ok {
		httpErr(w, http.StatusConflict, "kế hoạch cài đặt đã hết hạn hoặc đã dùng; hãy kiểm tra lại")
		return
	}
	ctx, cancel, ok := beginSetup(fullSetupID, fullSetupMax)
	if !ok {
		httpErr(w, http.StatusConflict, "đang có một lượt cài đặt khác")
		return
	}
	tools := make([]setup.Tool, 0, len(toolIDs))
	for _, id := range toolIDs {
		if tool, found := setup.Find(id); found && tool.Full {
			tools = append(tools, tool)
		}
	}
	go s.runFullSetup(ctx, cancel, tools)
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "count": len(tools)})
}

func (s *Server) createFullSetupGrant(toolIDs []string, now time.Time) string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic("không tạo được mã kế hoạch cài: " + err.Error())
	}
	id := base64.RawURLEncoding.EncodeToString(b)
	s.setupPlanMu.Lock()
	for oldID, grant := range s.setupPlans {
		if now.After(grant.Expires) {
			delete(s.setupPlans, oldID)
		}
	}
	s.setupPlans[id] = fullSetupGrant{ToolIDs: append([]string(nil), toolIDs...), Expires: now.Add(fullPlanTTL)}
	s.setupPlanMu.Unlock()
	return id
}

func (s *Server) consumeFullSetupGrant(id string, now time.Time) ([]string, bool) {
	s.setupPlanMu.Lock()
	defer s.setupPlanMu.Unlock()
	grant, ok := s.setupPlans[id]
	delete(s.setupPlans, id)
	if !ok || now.After(grant.Expires) {
		return nil, false
	}
	return append([]string(nil), grant.ToolIDs...), true
}

func (s *Server) runFullSetup(ctx context.Context, cancel context.CancelFunc, tools []setup.Tool) {
	defer endSetup(fullSetupID, cancel)
	emit := func(event map[string]any) {
		event["batch"] = fullSetupID
		s.Hub.Broadcast("setup", event)
	}

	for i, tool := range tools {
		status := s.setupToolStatus(ctx, tool)
		base := map[string]any{"tool": tool.ID, "index": i + 1, "total": len(tools)}
		if status.Installed {
			base["state"] = "skipped"
			base["line"] = "✓ " + tool.Label + " đã có — bỏ qua"
			emit(base)
			continue
		}

		plan, err := setup.BuildPlan(tool, "install", s.DataDir, filepath.Join(s.DataDir, "tmp"))
		if err != nil {
			s.emitFullError(emit, base, tool, err.Error())
			return
		}
		base["state"] = "running"
		base["line"] = "▶ " + plan.Cmds[0]
		emit(base)
		err = setup.Run(ctx, plan, func(line string) {
			emit(map[string]any{"batch": fullSetupID, "tool": tool.ID, "index": i + 1,
				"total": len(tools), "state": "running", "line": line})
		})
		if err != nil {
			msg := err.Error()
			if ctx.Err() == context.Canceled {
				msg = "đã hủy theo yêu cầu"
			} else if ctx.Err() == context.DeadlineExceeded {
				msg = "quá thời gian cài đặt cho phép"
			}
			s.emitFullError(emit, base, tool, msg)
			return
		}
		util.AugmentPATH()
		invalidateToolsCache()
		verifyCtx, verifyCancel := context.WithTimeout(ctx, 30*time.Second)
		verified := s.setupToolStatus(verifyCtx, tool)
		verifyCancel()
		if !verified.Installed {
			s.emitFullError(emit, base, tool,
				"bộ cài đã chạy xong nhưng Biz Studio chưa tìm thấy công cụ; hãy khởi động lại Biz Studio rồi bấm thử lại")
			return
		}
		emit(map[string]any{"batch": fullSetupID, "tool": tool.ID, "index": i + 1,
			"total": len(tools), "state": "done", "line": "✓ " + tool.Label + " đã cài xong"})
	}

	claude, _ := setup.Find("claude")
	claudeStatus := s.setupToolStatus(ctx, claude)
	emit(map[string]any{"tool": "", "state": "done", "needsLogin": claudeStatus.NeedsLogin,
		"line": "✅ Đã hoàn tất cài các thành phần còn thiếu"})
}

func (s *Server) emitFullError(emit func(map[string]any), base map[string]any, tool setup.Tool, msg string) {
	s.Log("error", "setup", "Cài Full dừng ở "+tool.Label+": "+msg)
	base["state"] = "error"
	base["error"] = msg
	base["manual"] = tool.Manual
	emit(base)
}
