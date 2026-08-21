package server

import (
	"context"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"bizstudio/internal/setup"
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
	Installed bool   `json:"installed"`
	Detail    string `json:"detail"`
}

func (s *Server) routesSetup(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/setup/tools", s.handleSetupTools)

	// POST /api/setup/{id}?action=install|update — chạy nền, tiến trình phát qua
	// SSE. Trả ngay để trình duyệt không phải giữ một kết nối treo 30 phút.
	mux.HandleFunc("POST /api/setup/{id}", s.handleSetupRun)

	mux.HandleFunc("POST /api/setup/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
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
	cfg := s.st.Settings()
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	tools := setup.Tools()
	out := make([]toolStatus, len(tools))
	var wg sync.WaitGroup
	for i, t := range tools {
		wg.Add(1)
		// Chạy song song: dò venv phải khởi động Python, tuần tự là mấy giây.
		go func(i int, t setup.Tool) {
			defer wg.Done()
			var c toolCheck
			switch t.ID {
			case "ffmpeg":
				c = checkBinVersion(ctx, "ffmpeg", "-version")
			case "ytdlp":
				c = checkYtdlp(ctx, binOrDefault(cfg.YtdlpBin, "yt-dlp"))
			case "chrome":
				c = checkChrome(ctx, cfg)
			case "vieneu":
				c = s.checkVieNeu(ctx)
			case "whisper":
				c = s.checkWhisper(ctx)
			default:
				c = toolCheck{Detail: "chưa có cách kiểm tra"}
			}
			out[i] = toolStatus{Tool: t, Installed: c.OK, Detail: c.Detail}
		}(i, t)
	}
	wg.Wait()
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSetupRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tool, ok := setup.Find(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "không có công cụ %q", id)
		return
	}
	action := r.URL.Query().Get("action")
	if action == "" {
		action = "install"
	}

	plan, err := setup.BuildPlan(tool, action, s.DataDir, filepath.Join(s.DataDir, "tmp"))
	if err != nil {
		// Không cài tự động được (thiếu brew/winget, cần sudo…) — đây là hướng
		// dẫn cho người dùng chứ không phải sự cố máy chủ.
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}

	runningMu.Lock()
	if _, busy := running[id]; busy {
		runningMu.Unlock()
		httpErr(w, http.StatusConflict, "%s đang được cài — chờ xong rồi thử lại", tool.Label)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), installMax)
	running[id] = cancel
	runningMu.Unlock()

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
		cancel()
		runningMu.Lock()
		delete(running, t.ID)
		runningMu.Unlock()
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
	s.Log("info", "setup", verb+" "+t.Label+" thành công")
	emit(map[string]any{"state": "done"})
}
