//go:build windows

package agent

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestWindowsAgentChildHelper(t *testing.T) {
	if os.Getenv("BIZSTUDIO_WINDOWS_AGENT_HELPER") != "1" {
		return
	}
	mode, root := os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	if mode == "child" {
		_ = os.WriteFile(filepath.Join(root, "child-started"), []byte("yes"), 0o600)
		time.Sleep(750 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(root, "escaped-child"), []byte("must not exist"), 0o600)
		os.Exit(0)
	}
	cmd := windowsAgentHelper(root, "child")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		os.Exit(8)
	}
	time.Sleep(time.Minute)
	os.Exit(0)
}

func windowsAgentHelper(root, mode string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^TestWindowsAgentChildHelper$", "--", mode, root)
	cmd.Env = append(os.Environ(), "BIZSTUDIO_WINDOWS_AGENT_HELPER=1")
	return cmd
}

func TestWindowsAgentCannotRunBeforeJobAssignment(t *testing.T) {
	root := t.TempDir()
	cmd := windowsAgentHelper(root, "parent")
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	cmd.WaitDelay = 2 * time.Second
	prepareAgentProcess(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	// Force the exact former race window, not just a statistical fast spawn.
	time.Sleep(150 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(root, "child-started")); !os.IsNotExist(err) {
		t.Fatalf("agent ran before job assignment: %v", err)
	}
	tree, err := attachAgentProcess(cmd.Process)
	if err != nil {
		t.Fatal(err)
	}
	defer tree.close()
	waitAgentCondition(t, func() bool { _, err := os.Stat(filepath.Join(root, "child-started")); return err == nil })
	tree.terminate()
	_ = cmd.Wait()
	time.Sleep(time.Second)
	if _, err := os.Stat(filepath.Join(root, "escaped-child")); !os.IsNotExist(err) {
		t.Fatalf("child escaped job: %v", err)
	}
}

func TestWindowsStopPreventsImmediateChildDelayedWrite(t *testing.T) {
	r, p, _ := testRunner(t)
	root := filepath.Join(r.dataDir, "projects", p.ID)
	r.commandBuilder = func(string, string, string) (*exec.Cmd, io.ReadCloser, *bytes.Buffer, error) {
		cmd := windowsAgentHelper(root, "parent")
		var errBuf bytes.Buffer
		cmd.Stderr = &errBuf
		out, err := cmd.StdoutPipe()
		return cmd, out, &errBuf, err
	}
	s, err := r.Start(p.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Stop(s.ID)
	waitAgentCondition(t, func() bool { _, err := os.Stat(filepath.Join(root, "child-started")); return err == nil })
	if err := r.Stop(s.ID); err != nil {
		t.Fatal(err)
	}
	waitAgentIdle(t, r, s.ID)
	time.Sleep(time.Second)
	if _, err := os.Stat(filepath.Join(root, "escaped-child")); !os.IsNotExist(err) {
		t.Fatalf("stopped child wrote after release: %v", err)
	}
}
