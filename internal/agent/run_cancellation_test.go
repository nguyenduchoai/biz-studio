package agent

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"bizstudio/internal/agentsdk"
)

func TestStopKillsDescendantBeforeProjectLeaseRelease(t *testing.T) {
	r, p, _ := testRunner(t)
	root := filepath.Join(r.dataDir, "projects", p.ID)
	r.commandBuilder = helperBuilder(t, "tree", root)
	s, err := r.Start(p.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = r.Stop(s.ID)
		raw, _ := os.ReadFile(filepath.Join(root, "child-pid"))
		pid, _ := strconv.Atoi(string(raw))
		if pid > 0 {
			if proc, err := os.FindProcess(pid); err == nil {
				_ = proc.Kill()
			}
		}
	})
	waitAgentCondition(t, func() bool {
		info, err := os.Stat(filepath.Join(root, "child-tick"))
		return err == nil && info.Size() > 2
	})
	if err := r.Stop(s.ID); err != nil {
		t.Fatal(err)
	}
	waitAgentIdle(t, r, s.ID)
	info, err := os.Stat(filepath.Join(root, "child-tick"))
	if err != nil {
		t.Fatal(err)
	}
	before := info.Size()
	time.Sleep(150 * time.Millisecond)
	info, err = os.Stat(filepath.Join(root, "child-tick"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != before {
		t.Fatal("descendant still writing after lease release")
	}
	got, _ := r.st.Session(s.ID)
	if got.Status != "stopped" {
		t.Fatalf("stop status=%s", got.Status)
	}
}

func TestStopDuringOutputValidationCannotBecomeDone(t *testing.T) {
	r, p, _ := testRunner(t)
	r.commandBuilder = helperBuilder(t, "success", filepath.Join(r.dataDir, "projects", p.ID))
	validating := make(chan struct{})
	r.validateVideo = func(ctx context.Context, _ string) error { close(validating); <-ctx.Done(); return ctx.Err() }
	s, err := r.Start(p.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-validating:
	case <-time.After(8 * time.Second):
		t.Fatal("no validation")
	}
	if err := r.Stop(s.ID); err != nil {
		t.Fatal(err)
	}
	waitAgentIdle(t, r, s.ID)
	got, _ := r.st.Session(s.ID)
	project, _ := r.st.Project(p.ID)
	if got.Status != "stopped" || project.Status != "draft" {
		t.Fatalf("stop became completion: %+v %+v", got, project)
	}
}

func TestResumeRejectsBackendChangeAndKeepsSession(t *testing.T) {
	r, p, s := testRunner(t)
	s.Backend = ""
	s.ClaudeSessionID = "legacy-cli"
	s.Status = "error"
	r.st.SaveSession(&s)
	cfg := r.st.Settings()
	cfg.ClaudeBackend = "sdk"
	r.st.SaveSettings(cfg)
	if err := r.Resume(s.ID, "continue"); err == nil || !strings.Contains(err.Error(), "backend") {
		t.Fatalf("mixed session backends: %v", err)
	}
	got, _ := r.st.Session(s.ID)
	if got.Status != "error" || got.ClaudeSessionID != "legacy-cli" {
		t.Fatalf("changed rejected session: %+v", got)
	}
	release, ok := r.st.ProjectWork.TryAcquire(p.ID)
	if !ok {
		t.Fatal("rejected backend leaked lease")
	}
	release()
}

func TestInvocationUsesSettingsSnapshot(t *testing.T) {
	r, p, s := testRunner(t)
	cfg := r.st.Settings()
	cfg.ClaudeBackend = "cli"
	cfg.ClaudeBin = "original-claude"
	cfg.ClaudeSDKBudgetUSD = 2
	release, ok := r.st.ProjectWork.TryAcquire(p.ID)
	if !ok {
		t.Fatal("lease")
	}
	defer release()
	state := r.reserve(s.ID, release, cfg)
	defer state.cancel()
	changed := cfg
	changed.ClaudeBackend = "sdk"
	changed.ClaudeBin = "replacement"
	changed.ClaudeSDKBudgetUSD = 20
	r.st.SaveSettings(changed)
	cmd, out, _, err := r.buildCmdWithSettings(p.ID, "render", "", state.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if cmd.Args[0] != "original-claude" || state.cfg.ClaudeSDKBudgetUSD != 2 {
		t.Fatalf("changed in-flight config: args=%v", cmd.Args)
	}
}

func TestSDKRuntimeInstallAndSessionExcludeEachOther(t *testing.T) {
	r, p, _ := testRunner(t)
	cfg := r.st.Settings()
	cfg.ClaudeBackend = "sdk"
	r.st.SaveSettings(cfg)
	releaseInstall, ok := agentsdk.TryInstallRuntime(r.dataDir)
	if !ok {
		t.Fatal("installer lease")
	}
	if _, err := r.Start(p.ID, ""); !errors.Is(err, ErrBusy) {
		t.Fatalf("SDK runtime conflict error=%v", err)
	}
	releaseInstall()
	entered, proceed := make(chan struct{}), make(chan struct{})
	r.commandBuilder = func(string, string, string) (*exec.Cmd, io.ReadCloser, *bytes.Buffer, error) {
		close(entered)
		<-proceed
		return nil, nil, nil, errors.New("test stopped")
	}
	s, err := r.Start(p.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	if s.Backend != "sdk" {
		t.Fatalf("SDK session recorded as %q", s.Backend)
	}
	if release, ok := agentsdk.TryInstallRuntime(r.dataDir); ok {
		release()
		t.Fatal("SDK runtime can be replaced during run")
	}
	_ = r.Stop(s.ID)
	close(proceed)
	waitAgentIdle(t, r, s.ID)
	release, ok := agentsdk.TryInstallRuntime(r.dataDir)
	if !ok {
		t.Fatal("SDK runtime lease leaked")
	}
	release()
}

func TestLeaderExitTerminatesChildHoldingBothOutputPipes(t *testing.T) {
	r, p, _ := testRunner(t)
	root := filepath.Join(r.dataDir, "projects", p.ID)
	r.commandBuilder = helperBuilder(t, "inherited-pipes", root)
	r.validateVideo = func(context.Context, string) error { return nil }
	s, err := r.Start(p.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = r.Stop(s.ID)
		raw, _ := os.ReadFile(filepath.Join(root, "child-pid"))
		pid, _ := strconv.Atoi(string(raw))
		if pid > 0 {
			if proc, err := os.FindProcess(pid); err == nil {
				_ = proc.Kill()
			}
		}
	})
	// Default watchdog is2h; this must finish from leaderexit within8s.
	waitAgentIdle(t, r, s.ID)
	got, _ := r.st.Session(s.ID)
	if got.Status != "done" {
		t.Fatalf("buffered result lost after leaderexit: %+v", got)
	}
	info, err := os.Stat(filepath.Join(root, "child-tick"))
	if err != nil {
		t.Fatal(err)
	}
	before := info.Size()
	time.Sleep(150 * time.Millisecond)
	info, err = os.Stat(filepath.Join(root, "child-tick"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != before {
		t.Fatal("inherited-pipe child survived successful completion")
	}
}
