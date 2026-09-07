package server

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"bizstudio/internal/updater"
)

func (s *Server) routesUpdate(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/update", s.handleUpdateInfo)
	mux.HandleFunc("POST /api/update/download", s.handleUpdateDownload)
	mux.HandleFunc("POST /api/update/apply", s.handleUpdateApply)
}

func (s *Server) handleUpdateInfo(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	info, err := s.Updater.Info(ctx)
	if err != nil {
		httpErr(w, http.StatusBadGateway, "%s", err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleUpdateDownload(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	info, err := s.Updater.Prepare(ctx)
	if err != nil {
		httpErr(w, http.StatusBadGateway, "%s", err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleUpdateApply(w http.ResponseWriter, _ *http.Request) {
	if detail := s.st.PersistenceError(); detail != "" {
		httpErr(w, http.StatusInsufficientStorage, "chưa thể cập nhật vì dữ liệu chưa lưu được: %s", detail)
		return
	}
	if reason := s.updateBusyReason(); reason != "" {
		httpErr(w, http.StatusConflict, "%s", reason)
		return
	}
	stage, err := s.Updater.Stage()
	if err != nil {
		httpErr(w, http.StatusConflict, "%s", err)
		return
	}
	// Reserve the setup slot until exit so another update or installer cannot
	// start while this helper waits for the old application to release files.
	_, cancel, acquired := beginSetup("app-update", time.Minute)
	if !acquired {
		httpErr(w, http.StatusConflict, "đang cài đặt thành phần; chờ hoàn tất trước khi cập nhật ứng dụng")
		return
	}
	stage.LaunchArgs = updateLaunchArgs(s.Port, s.DataDir, os.Args[1:])
	s.Log("info", "update", "Đã xác minh gói cập nhật; chuẩn bị cài bản "+stage.Tag)
	if detail := s.st.PersistenceError(); detail != "" {
		endSetup("app-update", cancel)
		httpErr(w, http.StatusInsufficientStorage, "chưa thể cập nhật vì dữ liệu chưa lưu được: %s", detail)
		return
	}
	if err := updater.Start(stage); err != nil {
		endSetup("app-update", cancel)
		httpErr(w, http.StatusInternalServerError, "%s", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "version": stage.Tag})
	go func() {
		time.Sleep(time.Second)
		os.Exit(0)
	}()
}

func (s *Server) updateBusyReason() string {
	if SetupInProgress() {
		return "đang cài đặt thành phần; chờ hoàn tất trước khi cập nhật ứng dụng"
	}
	if s.ideaRunner().Running() {
		return "hàng đợi sản xuất đang bật; dừng hàng đợi trước khi cập nhật"
	}
	for _, job := range s.st.Jobs() {
		if job.Status == "running" || job.Status == "queued" {
			return "đang có tác vụ xử lý; chờ hoàn tất hoặc dừng tác vụ trước khi cập nhật"
		}
	}
	for _, project := range s.st.Projects() {
		for _, session := range s.st.SessionsByProject(project.ID) {
			if session.Status == "running" {
				return "phiên AI đang chạy; chờ hoàn tất hoặc dừng phiên trước khi cập nhật"
			}
		}
	}
	for _, idea := range s.st.Ideas() {
		if idea.Status == "producing" {
			return "hàng đợi đang sản xuất video; chờ hoàn tất trước khi cập nhật"
		}
	}
	return ""
}

func updateLaunchArgs(port int, dataDir string, args []string) []string {
	out := []string{"-port", strconv.Itoa(port), "-data", dataDir}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if arg == "-data" || arg == "--data" || arg == "-port" || arg == "--port" {
			i++ // Their values may themselves look like flags.
			continue
		}
		if arg == "-window" || arg == "--window" {
			out = append(out, "-window=true")
		} else if strings.HasPrefix(arg, "-window=") || strings.HasPrefix(arg, "--window=") {
			out = append(out, "-"+strings.TrimLeft(arg, "-"))
		}
	}
	return out
}
