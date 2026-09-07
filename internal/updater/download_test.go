package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPackageDownloadHasLongerBudgetThanReleaseMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(60 * time.Millisecond):
			_, _ = w.Write([]byte("package"))
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	m := New("2.14.1", t.TempDir())
	m.client = srv.Client()
	m.client.Timeout = 10 * time.Millisecond
	if err := m.downloadTo(context.Background(), asset{Name: "package.zip", BrowserDownloadURL: srv.URL}, io.Discard, 100); err != nil {
		t.Fatalf("package inherited short metadata budget: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := m.downloadTo(ctx, asset{Name: "package.zip", BrowserDownloadURL: srv.URL}, io.Discard, 100); err == nil {
		t.Fatal("download must still obey caller cancellation")
	}
}

func TestVerifiedArchiveCanBeDownloadedAgainAndFailedRetryPreservesFile(t *testing.T) {
	payload := []byte("verified archive contents")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(payload) }))
	defer srv.Close()
	m := New("2.14.1", t.TempDir())
	m.client = srv.Client()
	path := filepath.Join(t.TempDir(), "release.zip")
	sum := sha256.Sum256(payload)
	a := asset{Name: "release.zip", BrowserDownloadURL: srv.URL, Size: int64(len(payload))}
	for i := 0; i < 2; i++ {
		if err := m.downloadArchive(context.Background(), a, path, hex.EncodeToString(sum[:])); err != nil {
			t.Fatalf("download %d: %v", i, err)
		}
	}
	if err := m.downloadArchive(context.Background(), a, path, "incorrect checksum"); err == nil {
		t.Fatal("checksum mismatch accepted")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(payload) {
		t.Fatalf("valid previous archive lost: %q, %v", got, err)
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 1 {
		t.Fatalf("partial downloads leaked: %v, %v", files, err)
	}
}

func TestDownloadRejectsTruncatedAndOversizedPackage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("short")) }))
	defer srv.Close()
	m := New("2.14.1", t.TempDir())
	m.client = srv.Client()
	for _, tc := range []struct{ size, limit int64 }{{10, 100}, {0, 3}} {
		if err := m.downloadTo(context.Background(), asset{Name: "release.zip", BrowserDownloadURL: srv.URL, Size: tc.size}, io.Discard, tc.limit); err == nil {
			t.Fatalf("size=%d limit=%d: invalid download accepted", tc.size, tc.limit)
		}
	}
}
