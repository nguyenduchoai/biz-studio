package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataRouteNeverServesDatabase(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/data/db.json", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Host = "127.0.0.1:6868"
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("/data/db.json = %d, muốn 404", rec.Code)
	}
}

func TestDataRouteServesAllowedProjectMedia(t *testing.T) {
	s := newTestServer(t)
	path := filepath.Join(s.DataDir, "projects", "p1", "outputs", "final.mp4")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/data/projects/p1/outputs/final.mp4", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Host = "127.0.0.1:6868"
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "video") {
		t.Fatalf("media hợp lệ = %d %q", rec.Code, rec.Body.String())
	}
}

func TestDataRouteServesGeneratedASSSubtitle(t *testing.T) {
	s := newTestServer(t)
	path := filepath.Join(s.DataDir, "projects", "p1", "outputs", "karaoke.ass")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[Script Info]"), 0o600); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/data/projects/p1/outputs/karaoke.ass", nil)
	req.RemoteAddr, req.Host = "127.0.0.1:12345", "127.0.0.1:6868"
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ASS hợp lệ = %d", rec.Code)
	}
}
