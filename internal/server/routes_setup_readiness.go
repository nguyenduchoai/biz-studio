package server

import (
	"context"
	"net/http"
	"time"

	"bizstudio/internal/setup"
)

const windowsPrepareID = "windows-firewall"

func (s *Server) handleWindowsSetupStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, setup.CheckWindowsReadiness(ctx))
}

func (s *Server) handleWindowsFirewallSetup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Confirmed bool `json:"confirmed"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	if !body.Confirmed {
		httpErr(w, http.StatusBadRequest, "cần xác nhận cho phép nhận file QR trong mạng nội bộ")
		return
	}
	ctx, cancel, ok := beginSetup(windowsPrepareID, 5*time.Minute)
	if !ok {
		httpErr(w, http.StatusConflict, "đang có một lượt cài đặt khác")
		return
	}
	defer endSetup(windowsPrepareID, cancel)
	s.Log("info", "setup", "Đang xin quyền Windows để cho phép nhận file QR trên mạng Private/Domain")
	if err := setup.ConfigureWindowsFirewall(ctx); err != nil {
		s.Log("error", "setup", "Cấu hình Windows Firewall thất bại: "+err.Error())
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	verifyCtx, verifyCancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer verifyCancel()
	status := setup.CheckWindowsReadiness(verifyCtx)
	s.Log("info", "setup", "Windows Firewall đã cho phép Biz Studio nhận file QR trên mạng Private/Domain")
	writeJSON(w, http.StatusOK, status)
}
