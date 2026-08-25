package server

import (
	"context"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"bizstudio/internal/setup"
	"bizstudio/internal/util"
)

// installMax — trần thời gian cho một lượt cài. faster-whisper kèm model
// large-v3 tải hơn 3 GB trên đường truyền chậm, nên đặt rộng tay.
const installMax = 60 * time.Minute

// running theo dõi các lượt cài đang chạy để không bấm hai lần thành hai tiến
// trình pip cùng ghi vào một venv (hỏng venv, và lỗi sinh ra thì vô nghĩa).
var (
	runningMu sync.Mutex
	running   = map[string]context.CancelFunc{}
)

// toolStatus — một công cụ kèm tình trạng trên máy này.
type toolStatus struct {
	setup.Tool
	Installed  bool   `json:"installed"`
	Ready      bool   `json:"ready"`
	NeedsLogin bool   `json:"needsLogin,omitempty"`
	Running    bool   `json:"running"`
	Detail     string `json:"detail"`
}

func (s *Server) routesSetup(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/setup/tools", s.handleSetupTools)
	mux.HandleFunc("GET /api/setup/full/plan", s.handleSetupFullPlan)
	mux.HandleFunc("POST /api/setup/full", s.handleSetupFullRun)

	// POST /api/setup/{id}?action=install|update — chạy nền, tiến trình phát qua
	// SSE. Trả ngay để trình duyệt không phải giữ một kết nối treo 30 phút.
	mux.HandleFunc("POST /api/setup/{id}", s.handleSetupRun)

	mux.HandleFunc("POST /api/setup/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		id := canonicalSetupID(r.PathValue("id"))
		runningMu.Lock()
		cancel, ok := running[id]
		runningMu.Unlock()
		if !ok {
			httpErr(w, http.StatusNotFound, "%s không có lượt cài nào đang chạy", id)
			return
		}
		cancel()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
}

// handleSetupTools trả danh mục công cụ kèm tình trạng cài đặt.
//
// Chỉ kiểm tra CỤC BỘ (chạy `--version`, thử import trong venv) — không gọi ra
// mạng. Trang Cấu hình gọi hàm này ngay khi mở, mà một lần chờ API bên ngoài
// trả lời là đủ để trang trông như bị treo.
func (s *Server) handleSetupTools(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, s.setupStatuses(ctx))
}

func (s *Server) handleSetupRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tool, ok := setup.Find(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "không có công cụ %q", id)
		return
	}
	id = tool.ID
	action := r.URL.Query().Get("action")
	if action == "" {
		action = "install"
	}

	ctx, cancel, ok := beginSetup(id, installMax)
	if !ok {
		httpErr(w, http.StatusConflict, "đang có một lượt cài đặt khác — chờ xong rồi thử lại")
		return
	}

	plan, err := setup.BuildPlan(tool, action, s.DataDir, filepath.Join(s.DataDir, "tmp"))
	if err != nil {
		endSetup(id, cancel)
		// Không cài tự động được (thiếu brew/winget, cần sudo…) — đây là hướng
		// dẫn cho người dùng chứ không phải sự cố máy chủ.
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}

	go s.runSetup(ctx, cancel, tool, plan)
	writeJSON(w, http.StatusOK, plan)
}

// runSetup chạy plan và phát từng dòng ra SSE dưới sự kiện "setup".
//
// Dùng context nền chứ không phải context của request: người dùng đóng tab hay
// chuyển trang giữa chừng thì việc cài vẫn phải chạy tiếp, không bỏ dở một venv
// đang cài nửa vời.
func (s *Server) runSetup(ctx context.Context, cancel context.CancelFunc, t setup.Tool, plan *setup.Plan) {
	defer func() {
		endSetup(t.ID, cancel)
	}()

	emit := func(ev map[string]any) {
		ev["tool"] = t.ID
		s.Hub.Broadcast("setup", ev)
	}
	verb := "Cài"
	if plan.Action == "update" {
		verb = "Cập nhật"
	}
	s.Log("info", "setup", verb+" "+t.Label+": "+plan.Cmds[0])
	emit(map[string]any{"line": "▶ " + plan.Cmds[0], "state": "running"})

	err := setup.Run(ctx, plan, func(line string) {
		emit(map[string]any{"line": line, "state": "running"})
	})
	if err != nil {
		msg := err.Error()
		if ctx.Err() == context.DeadlineExceeded {
			msg = "quá thời gian cho phép — mạng chậm hoặc bị treo"
		} else if ctx.Err() == context.Canceled {
			msg = "đã hủy theo yêu cầu"
		}
		s.Log("error", "setup", verb+" "+t.Label+" thất bại: "+msg)
		emit(map[string]any{"state": "error", "error": msg, "manual": t.Manual})
		return
	}
	util.AugmentPATH()
	invalidateToolsCache()
	verifyCtx, verifyCancel := context.WithTimeout(ctx, 30*time.Second)
	status := s.setupToolStatus(verifyCtx, t)
	verifyCancel()
	if !status.Installed {
		msg := "bộ cài đã chạy xong nhưng Biz Studio chưa tìm thấy công cụ; hãy khởi động lại Biz Studio rồi thử kiểm tra lại"
		s.Log("error", "setup", verb+" "+t.Label+" chưa xác minh được: "+status.Detail)
		emit(map[string]any{"state": "error", "error": msg, "manual": t.Manual, "restartRequired": true})
		return
	}
	s.Log("info", "setup", verb+" "+t.Label+" thành công")
	emit(map[string]any{"state": "done"})
}

func beginSetup(id string, max time.Duration) (context.Context, context.CancelFunc, bool) {
	runningMu.Lock()
	defer runningMu.Unlock()
	if len(running) > 0 {
		return nil, nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), max)
	running[id] = cancel
	return ctx, cancel, true
}

func endSetup(id string, cancel context.CancelFunc) {
	cancel()
	runningMu.Lock()
	delete(running, id)
	runningMu.Unlock()
}

func setupIsRunning(id string) bool {
	runningMu.Lock()
	defer runningMu.Unlock()
	_, ok := running[id]
	return ok
}

func canonicalSetupID(id string) string {
	if tool, ok := setup.Find(id); ok {
		return tool.ID
	}
	return id
}

// SetupInProgress cho vòng đời desktop biết không được tắt backend giữa lúc
// WinGet/PowerShell/pip còn đang ghi dở vào máy hoặc venv.
func SetupInProgress() bool {
	runningMu.Lock()
	defer runningMu.Unlock()
	return len(running) > 0
}
