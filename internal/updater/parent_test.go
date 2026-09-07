package updater

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestUpdaterChildExit(t *testing.T) {
	if os.Getenv("BIZSTUDIO_UPDATE_PARENT_TEST") != "1" {
		return
	}
	time.Sleep(200 * time.Millisecond)
	os.Exit(0)
}

func TestWaitForParentObservesExitAndTimeout(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestUpdaterChildExit$")
	cmd.Env = append(os.Environ(), "BIZSTUDIO_UPDATE_PARENT_TEST=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	started := time.Now()
	if err := waitForParent(cmd.Process.Pid, 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) < 150*time.Millisecond {
		t.Fatal("updater did not wait for the previous process to exit")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := waitForParent(os.Getpid(), time.Millisecond); err == nil {
		t.Fatal("live process must time out without replacing files")
	}
}
