package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bizstudio/internal/store"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("mở store: %v", err)
	}
	return New(st, t.TempDir(), 6868, 6869)
}

func TestMobileListenerDoesNotExposeControlAPI(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/setup/tools", nil)
	rec := httptest.NewRecorder()
	s.MobileHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("mobile /api/setup/tools = %d, muốn 404", rec.Code)
	}
}

func TestMobileListenerRequiresQRToken(t *testing.T) {
	s := newTestServer(t)
	for _, path := range []string{"/m/project-1", "/m/project-1?token=sai"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		s.MobileHandler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("GET %s = %d, muốn 403", path, rec.Code)
		}
	}
}

func TestMobileTokenIsBoundToProject(t *testing.T) {
	s := newTestServer(t)
	now := time.Unix(1_800_000_000, 0)
	a := s.mobileTokenForAt("project-a", now)
	b := s.mobileTokenForAt("project-b", now)
	if a == b || a == "" || b == "" {
		t.Fatalf("token chưa gắn theo project: a=%q b=%q", a, b)
	}
}

func TestMobileTokenExpires(t *testing.T) {
	s := newTestServer(t)
	now := time.Unix(1_800_000_000, 0)
	token := s.mobileTokenForAt("project-a", now)
	if !s.validMobileToken("project-a", token, now.Add(time.Minute)) {
		t.Fatal("token mới bị từ chối")
	}
	if s.validMobileToken("project-a", token, now.Add(mobileTokenTTL+time.Second)) {
		t.Fatal("token hết hạn vẫn được chấp nhận")
	}
	if s.validMobileToken("project-b", token, now) {
		t.Fatal("token dùng chéo dự án")
	}
}

func TestMobileUploadTokenCanOnlyBeReservedOnce(t *testing.T) {
	s := newTestServer(t)
	now := time.Unix(1_800_000_000, 0)
	token := s.mobileTokenForAt("project-a", now)
	if !s.reserveMobileToken(token, now) {
		t.Fatal("không reserve được token mới")
	}
	if s.reserveMobileToken(token, now) {
		t.Fatal("token replay vẫn được reserve")
	}
	s.releaseMobileToken(token)
	if !s.reserveMobileToken(token, now) {
		t.Fatal("token lỗi không được release để retry")
	}
}

func TestControlRejectsCrossSiteMutation(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/setup/ffmpeg", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Host = "127.0.0.1:6868"
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site POST = %d, muốn 403", rec.Code)
	}
}

func TestControlRejectsDNSRebindingHost(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/setup/ffmpeg", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Host = "evil.example"
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("DNS rebinding POST = %d, muốn 403", rec.Code)
	}
}
