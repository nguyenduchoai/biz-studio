package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"bizstudio/internal/store"
)

func TestMutationReturns507AndRollsBackWhenStoreCannotPersist(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := New(st, dir, 6868, 6869)
	offline := dir + "-offline"
	if err := os.Rename(dir, offline); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(offline) })
	req := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"name":"không được ACK"}`))
	req.RemoteAddr, req.Host = "127.0.0.1:12345", "127.0.0.1:6868"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://127.0.0.1:6868")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("mutation persist lỗi = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(st.Projects()) != 0 {
		t.Fatalf("mutation lỗi vẫn còn trong RAM: %#v", st.Projects())
	}
}
