package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMacLauncherPreservesLiteralPathsAndRestartArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX launcher")
	}
	dir := filepath.Join(t.TempDir(), "Cài đặt $APP 'quote' $(literal)", "Biz Studio.app", "Contents", "MacOS")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	launcher, err := os.ReadFile("macos-launcher.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "launcher"), launcher, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bizstudio"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(t.TempDir(), "Dữ liệu $DATA 'quote' $(literal)")
	cmd := exec.Command("/bin/bash", filepath.Join(dir, "launcher"), "-window=false", "-port", "17680", "-data", dataDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("launcher: %v: %s", err, out)
	}
	want := "-window=false\n-port\n17680\n-data\n" + dataDir + "\n"
	if !strings.HasSuffix(string(out), want) {
		t.Fatalf("restart args changed: %q, want suffix %q", out, want)
	}
}
