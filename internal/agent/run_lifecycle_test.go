package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The current Go test binary stands in for the provider CLI on every native
// platform. Never calls Claude or a network API, and does not require a shell.
func TestAgentSubprocess(t *testing.T) {
	if os.Getenv("BIZSTUDIO_AGENT_TEST_HELPER") != "1" {
		return
	}
	mode, root := os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	if mode == "child" {
		f, err := os.OpenFile(filepath.Join(root, "child-tick"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			os.Exit(8)
		}
		defer f.Close()
		for {
			_, _ = f.Write([]byte("."))
			_ = f.Sync()
			time.Sleep(20 * time.Millisecond)
		}
	}
	fmt.Println(`{"type":"system","subtype":"init","session_id":"test-resume-id"}`)
	if mode == "tree" || mode == "inherited-pipes" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestAgentSubprocess$", "--", "child", root)
		cmd.Env = os.Environ()
		if mode == "inherited-pipes" {
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		}
		if err := cmd.Start(); err != nil {
			os.Exit(8)
		}
		_ = os.WriteFile(filepath.Join(root, "child-pid"), []byte(fmt.Sprint(cmd.Process.Pid)), 0o600)
		if mode == "inherited-pipes" {
			for {
				if info, err := os.Stat(filepath.Join(root, "child-tick")); err == nil && info.Size() > 0 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
	if mode == "sleep" || mode == "tree" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	_ = os.WriteFile(filepath.Join(root, "outputs", "new.mp4"), []byte("new video"), 0o600)
	_ = os.WriteFile(filepath.Join(root, "meta.json"), []byte(`{"status":"done","output":"outputs/new.mp4"}`), 0o600)
	fmt.Println(`{"type":"result","subtype":"success","num_turns":2}`)
	if mode == "hold" {
		for {
			if _, err := os.Stat(filepath.Join(root, "allow-exit")); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	if mode == "exit7" {
		os.Exit(7)
	}
	os.Exit(0)
}

func helperBuilder(t *testing.T, mode, root string) func(string, string, string) (*exec.Cmd, io.ReadCloser, *bytes.Buffer, error) {
	t.Helper()
	return func(string, string, string) (*exec.Cmd, io.ReadCloser, *bytes.Buffer, error) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestAgentSubprocess$", "--", mode, root)
		cmd.Env = append(os.Environ(), "BIZSTUDIO_AGENT_TEST_HELPER=1")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		stdout, err := cmd.StdoutPipe()
		return cmd, stdout, &stderr, err
	}
}

func waitAgentCondition(t *testing.T, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("agent condition not met before deadline")
}

func waitAgentIdle(t *testing.T, r *Runner, id string) {
	t.Helper()
	waitAgentCondition(t, func() bool { r.mu.Lock(); defer r.mu.Unlock(); _, ok := r.active[id]; return !ok })
}

func TestLeaseAndRunningStatusLastThroughProcessExitAndValidation(t *testing.T) {
	r, p, _ := testRunner(t)
	root := filepath.Join(r.dataDir, "projects", p.ID)
	r.commandBuilder = helperBuilder(t, "hold", root)
	validationStarted, allowValidation := make(chan struct{}), make(chan struct{})
	r.validateVideo = func(ctx context.Context, _ string) error {
		close(validationStarted)
		select {
		case <-allowValidation:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s, err := r.Start(p.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Stop(s.ID) })
	waitAgentCondition(t, func() bool {
		for _, ev := range r.st.EventsBySession(s.ID) {
			if ev.Type == "result" {
				return true
			}
		}
		return false
	})
	assertBusy := func() {
		t.Helper()
		got, _ := r.st.Session(s.ID)
		if got.Status != "running" {
			t.Fatalf("premature status=%s", got.Status)
		}
		if release, ok := r.st.ProjectWork.TryAcquire(p.ID); ok {
			release()
			t.Fatal("lease released early")
		}
		if _, err := r.Start(p.ID, ""); err == nil {
			t.Fatal("concurrent project session accepted")
		}
		if err := r.Resume(s.ID, "again"); err == nil {
			t.Fatal("concurrent resume accepted")
		}
	}
	assertBusy()
	writeAgentOutput(t, root, "allow-exit", "yes")
	select {
	case <-validationStarted:
	case <-time.After(8 * time.Second):
		t.Fatal("validation never started")
	}
	assertBusy()
	close(allowValidation)
	waitAgentIdle(t, r, s.ID)
	got, _ := r.st.Session(s.ID)
	project, _ := r.st.Project(p.ID)
	if got.Status != "done" || project.Status != "done" || !strings.HasSuffix(project.OutputFile, "/outputs/new.mp4") {
		t.Fatalf("not completed: session=%+v project=%+v", got, project)
	}
	release, ok := r.st.ProjectWork.TryAcquire(p.ID)
	if !ok {
		t.Fatal("lease not released after validation")
	}
	release()
}

func TestActualNonzeroExitAfterResultFails(t *testing.T) {
	r, p, _ := testRunner(t)
	r.commandBuilder = helperBuilder(t, "exit7", filepath.Join(r.dataDir, "projects", p.ID))
	r.validateVideo = func(context.Context, string) error { t.Error("must not validate failed invocation"); return nil }
	s, err := r.Start(p.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	waitAgentIdle(t, r, s.ID)
	got, _ := r.st.Session(s.ID)
	project, _ := r.st.Project(p.ID)
	if got.Status != "error" || project.Status != "error" || got.ClaudeSessionID != "test-resume-id" {
		t.Fatalf("incorrect failure/resume state: %+v %+v", got, project)
	}
}

func TestImmediateStopBeforeProcessStarts(t *testing.T) {
	r, p, _ := testRunner(t)
	entered, proceed := make(chan struct{}), make(chan struct{})
	r.commandBuilder = func(string, string, string) (*exec.Cmd, io.ReadCloser, *bytes.Buffer, error) {
		close(entered)
		<-proceed
		return nil, nil, nil, errors.New("test launch canceled")
	}
	s, err := r.Start(p.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	if err := r.Stop(s.ID); err != nil {
		t.Fatal(err)
	}
	close(proceed)
	waitAgentIdle(t, r, s.ID)
	got, _ := r.st.Session(s.ID)
	project, _ := r.st.Project(p.ID)
	if got.Status != "stopped" || project.Status != "draft" {
		t.Fatalf("stop was overwritten: %+v %+v", got, project)
	}
}

func TestRunDeadlineCancelsProcessAndReleasesLease(t *testing.T) {
	r, p, _ := testRunner(t)
	r.runTimeout = 250 * time.Millisecond
	r.commandBuilder = helperBuilder(t, "sleep", filepath.Join(r.dataDir, "projects", p.ID))
	s, err := r.Start(p.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	waitAgentIdle(t, r, s.ID)
	got, _ := r.st.Session(s.ID)
	if got.Status != "error" {
		t.Fatalf("deadline status=%s", got.Status)
	}
	release, ok := r.st.ProjectWork.TryAcquire(p.ID)
	if !ok {
		t.Fatal("deadline leaked lease")
	}
	release()
}

func TestBusyProjectRejectsWithoutCreatingSession(t *testing.T) {
	r, p, s := testRunner(t)
	s.ClaudeSessionID = "resume"
	r.st.SaveSession(&s)
	release, _ := r.st.ProjectWork.TryAcquire(p.ID)
	defer release()
	before := len(r.st.SessionsByProject(p.ID))
	if _, err := r.Start(p.ID, ""); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy Start error=%v", err)
	}
	if err := r.Resume(s.ID, "continue"); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy Resume error=%v", err)
	}
	if len(r.st.SessionsByProject(p.ID)) != before {
		t.Fatal("rejection created phantom session")
	}
}

func TestResumeCannotReusePreviousOutput(t *testing.T) {
	r, p, _ := testRunner(t)
	r.commandBuilder = helperBuilder(t, "success", filepath.Join(r.dataDir, "projects", p.ID))
	r.validateVideo = func(context.Context, string) error { return nil }
	s, err := r.Start(p.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	waitAgentIdle(t, r, s.ID)
	first, _ := r.st.Project(p.ID)
	if first.Status != "done" {
		t.Fatalf("first run: %+v", first)
	}
	if err := r.Resume(s.ID, "please edit again"); err != nil {
		t.Fatal(err)
	}
	waitAgentIdle(t, r, s.ID)
	got, _ := r.st.Session(s.ID)
	project, _ := r.st.Project(p.ID)
	if got.Status != "error" || project.Status != "error" || project.OutputFile != first.OutputFile {
		t.Fatalf("stale output accepted or old file lost: %+v %+v", got, project)
	}
}
