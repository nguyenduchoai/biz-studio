package server

import (
	"context"
	"net/http"
	"os"
	"strconv"
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
	stage, err := s.Updater.Stage()
	if err != nil {
		httpErr(w, http.StatusConflict, "%s", err)
		return
	}
	stage.LaunchArgs = []string{"-port", strconv.Itoa(s.Port), "-data", s.DataDir}
	if err := updater.Start(stage); err != nil {
		httpErr(w, http.StatusInternalServerError, "%s", err)
		return
	}
	s.Log("info", "update", "Đã xác minh gói cập nhật; ứng dụng sẽ khởi động lại để cài bản "+stage.Tag)
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "version": stage.Tag})
	go func() {
		time.Sleep(time.Second)
		os.Exit(0)
	}()
}
