package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"bizstudio/internal/util"
)

func TestIsBizStudioAtRejectsUnrelatedListener(t *testing.T) {
	other := httptest.NewServer(http.NotFoundHandler())
	defer other.Close()

	if isBizStudioAt(other.URL, "expected") {
		t.Fatal("listener HTTP khác bị nhận nhầm là Biz Studio")
	}
}

func TestIsBizStudioAtAcceptsInstanceMarker(t *testing.T) {
	dataID := "data-123"
	studio := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/instance" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"app":"bizstudio","dataID":"` + dataID + `"}`))
	}))
	defer studio.Close()

	if !isBizStudioAt(studio.URL, dataID) {
		t.Fatal("không nhận ra Biz Studio qua marker /api/instance")
	}
}

func TestListenControlFallsBackWhenPreferredPortIsBusy(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	preferred := busy.Addr().(*net.TCPAddr).Port

	ln, port, err := listenControl(preferred)
	if err != nil {
		t.Fatalf("listenControl: %v", err)
	}
	defer ln.Close()
	if port == preferred {
		t.Fatalf("vẫn dùng cổng đang bận %d", preferred)
	}
	if host := ln.Addr().(*net.TCPAddr).IP; !host.IsLoopback() {
		t.Fatalf("control listener không nằm trên loopback: %s", host)
	}
}

func TestRunningInstanceURLUsesSavedFallbackPort(t *testing.T) {
	dataDir := t.TempDir()
	dataID := util.DataDirID(dataDir)
	studio := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/instance" {
			_, _ = w.Write([]byte(`{"app":"bizstudio","dataID":"` + dataID + `"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer studio.Close()

	if err := os.WriteFile(filepath.Join(dataDir, instanceFileName), []byte(`{"url":"`+studio.URL+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := runningInstanceURL(dataDir, 1); got != studio.URL {
		t.Fatalf("runningInstanceURL = %q, muốn %q", got, studio.URL)
	}
}

func TestInstanceLockRejectsSecondWriter(t *testing.T) {
	dir := t.TempDir()
	first, ok, err := acquireInstanceLock(dir)
	if err != nil || !ok {
		t.Fatalf("khóa thứ nhất: ok=%v err=%v", ok, err)
	}
	defer first.Close()
	second, ok, err := acquireInstanceLock(dir)
	if err != nil {
		t.Fatalf("khóa thứ hai: %v", err)
	}
	if ok || second != nil {
		t.Fatal("writer thứ hai vẫn lấy được cùng khóa dữ liệu")
	}
}
