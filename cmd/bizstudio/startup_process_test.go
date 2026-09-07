package main

import (
	"context"
	"flag"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Exercise the actual entry point in a separate native process. Unit tests of
// listenControl alone cannot catch startup/lock/marker ordering regressions.
func TestNativeStartupAndReopenWithBusyPort(t *testing.T) {
	busy, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port
	dir := filepath.Join(t.TempDir(), "Dữ liệu dự án")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	first := startupHelper(ctx, dir, busy.Addr().String())
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = first.Process.Kill()
		_ = first.Wait()
	})
	var url string
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		url = runningInstanceURL(dir, port)
		if url != "" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if url == "" {
		log, _ := os.ReadFile(filepath.Join(dir, "bizstudio-startup.log"))
		t.Fatalf("native startup did not become ready: %s", log)
	}
	second := startupHelper(ctx, dir, busy.Addr().String())
	if output, err := second.CombinedOutput(); err != nil {
		t.Fatalf("reopening same data directory failed: %v\n%s", err, output)
	}
	if reopened := runningInstanceURL(dir, port); reopened != url {
		t.Fatalf("reopen replaced or lost original instance: before=%q after=%q", url, reopened)
	}
	if lock, acquired, err := acquireInstanceLock(dir); err != nil || acquired {
		if lock != nil {
			_ = lock.Close()
		}
		t.Fatalf("original process no longer protects database: acquired=%t error=%v", acquired, err)
	}
}

func startupHelper(ctx context.Context, dir, addr string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNativeStartupProcessHelper$")
	cmd.Env = append(os.Environ(), "BIZSTUDIO_TEST_STARTUP_DIR="+dir, "BIZSTUDIO_TEST_STARTUP_ADDR="+addr)
	return cmd
}

func TestNativeStartupProcessHelper(t *testing.T) {
	dir := os.Getenv("BIZSTUDIO_TEST_STARTUP_DIR")
	if dir == "" {
		return
	}
	_, port, err := net.SplitHostPort(os.Getenv("BIZSTUDIO_TEST_STARTUP_ADDR"))
	if err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"bizstudio", "-window=false", "-port", port, "-data", dir}
	flag.CommandLine = flag.NewFlagSet("bizstudio", flag.ExitOnError)
	main()
}
