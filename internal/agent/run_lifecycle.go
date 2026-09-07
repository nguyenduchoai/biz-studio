package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"bizstudio/internal/store"
)

type activeRun struct {
	ctx     context.Context
	cancel  context.CancelFunc
	release func()
	cfg     store.Settings
}

// Reserve synchronously, before the goroutine or subprocess starts, so an
// immediate Stop works and a simultaneous Resume cannot launch a second writer.
func (r *Runner) reserve(sessionID string, release func(), cfg store.Settings) *activeRun {
	ctx, cancel := context.WithTimeout(context.Background(), r.runTimeout)
	state := &activeRun{ctx: ctx, cancel: cancel, release: release, cfg: cfg}
	r.mu.Lock()
	defer r.mu.Unlock()
	if sess, ok := r.st.Session(sessionID); ok {
		sess.Status = "running"
		sess.EndedAt = time.Time{}
		r.st.SaveSession(&sess)
		r.broadcast("session", sess)
		if p, ok := r.st.Project(sess.ProjectID); ok {
			p.Status = "running"
			if p.Progress < 1 || p.Progress >= 6 {
				p.Progress = 1
			}
			r.st.SaveProject(&p)
			r.broadcast("project", p)
		}
	}
	r.active[sessionID] = state
	return state
}

func (r *Runner) run(state *activeRun, sessionID, projectID, prompt, resumeID string) {
	defer func() {
		state.cancel()
		r.mu.Lock()
		state.release()
		delete(r.active, sessionID)
		r.mu.Unlock()
	}()
	snapshotCtx, cancel := context.WithTimeout(state.ctx, 10*time.Minute)
	before, err := r.snapshotOutputs(snapshotCtx, projectID)
	cancel()
	if err != nil {
		r.failSession(sessionID, projectID, "Không kiểm tra được kết quả trước lượt chạy: "+err.Error())
		return
	}
	builder := r.commandBuilder
	if builder == nil {
		builder = func(projectID, prompt, resumeID string) (*exec.Cmd, io.ReadCloser, *bytes.Buffer, error) {
			return r.buildCmdWithSettings(projectID, prompt, resumeID, state.cfg)
		}
	}
	cmd, stdout, stderr, err := builder(projectID, prompt, resumeID)
	if err != nil {
		r.failSession(sessionID, projectID, err.Error())
		return
	}
	defer stdout.Close()
	pipes, err := ownAgentPipes(cmd, stdout, stderr)
	if err != nil {
		r.failSession(sessionID, projectID, "Không mở được luồng Claude: "+err.Error())
		return
	}
	defer pipes.close()
	if err := state.ctx.Err(); err != nil {
		r.failSession(sessionID, projectID, err.Error())
		return
	}
	prepareAgentProcess(cmd)
	// Bound an inherited stdin writer too; output pipes are owned separately.
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Start(); err != nil {
		r.failSession(sessionID, projectID, "Không khởi động được Claude: "+err.Error())
		return
	}
	pipes.closeWriters()
	tree, err := attachAgentProcess(cmd.Process)
	if err != nil {
		_ = cmd.Process.Kill()
		pipes.closeReaders()
		_ = cmd.Wait()
		r.failSession(sessionID, projectID, "Không quản lý được tiến trình Claude an toàn: "+err.Error())
		return
	}
	defer tree.close()
	stderrDone := make(chan struct{})
	go func() { defer close(stderrDone); _, _ = io.Copy(pipes.errTarget, pipes.stderr) }()
	waitDone := make(chan error, 1)
	go func() { err := cmd.Wait(); tree.terminate(); waitDone <- err }()
	watchDone := make(chan struct{})
	watchExited := make(chan struct{})
	go func() {
		defer close(watchExited)
		select {
		case <-state.ctx.Done():
			tree.terminate()
			pipes.closeReaders()
		case <-watchDone:
		}
	}()
	result, streamErr := r.readStream(sessionID, projectID, pipes.stdout)
	if streamErr != nil {
		tree.terminate()
	}
	waitErr := <-waitDone
	<-stderrDone
	close(watchDone)
	<-watchExited
	if streamErr != nil {
		waitErr = errors.Join(waitErr, fmt.Errorf("lỗi đọc stream Claude: %w", streamErr))
	}
	r.finishRun(state.ctx, sessionID, projectID, result, waitErr, stderr, before)
}
