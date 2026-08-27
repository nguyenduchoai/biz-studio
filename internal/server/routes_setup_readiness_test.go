package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"bizstudio/internal/setup"
)

func TestWindowsSetupStatusContract(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/setup/windows/status", nil)
	markLocalControlRequest(req)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var status setup.WindowsReadiness
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && status.Supported {
		t.Fatalf("non-Windows lại báo supported: %+v", status)
	}
}

func TestWindowsFirewallSetupRequiresExplicitConfirmation(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/setup/windows/firewall", strings.NewReader(`{"confirmed":false}`))
	markLocalControlRequest(req)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestWindowsFirewallSetupIsUnavailableOffWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("không được bật UAC trong unit test Windows")
	}
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/setup/windows/firewall", strings.NewReader(`{"confirmed":true}`))
	markLocalControlRequest(req)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "chỉ hỗ trợ trên Windows") {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func markLocalControlRequest(req *http.Request) {
	req.Host = "127.0.0.1:6868"
	req.RemoteAddr = "127.0.0.1:12345"
}
