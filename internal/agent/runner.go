// Package agent — Claude CLI session runner cho tính năng "Phiên AI" edit video.
// Chạy claude CLI (subscription, không API key) trong thư mục dự án, parse
// stream-json và phát sự kiện SSE qua broadcast.
package agent

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bizstudio/internal/agentsdk"
	"bizstudio/internal/media"
	"bizstudio/internal/store"
)

// ErrBusy is an expected admission conflict, not a provider/startup failure.
var ErrBusy = errors.New("tác vụ đang bận")

// Runner quản lý các phiên AI (Claude CLI) đang chạy.
type Runner struct {
	st             *store.Store
	broadcast      func(string, any)
	dataDir        string
	mu             sync.Mutex
	active         map[string]*activeRun
	runTimeout     time.Duration
	validateVideo  func(context.Context, string) error
	commandBuilder func(string, string, string) (*exec.Cmd, io.ReadCloser, *bytes.Buffer, error)
}

// New tạo Runner mới.
func New(st *store.Store, broadcast func(string, any), dataDir string) *Runner {
	return &Runner{
		st:            st,
		broadcast:     broadcast,
		dataDir:       dataDir,
		active:        map[string]*activeRun{},
		runTimeout:    2 * time.Hour,
		validateVideo: media.ValidateVideo,
	}
}

// Start tạo phiên AI mới cho dự án và chạy Claude CLI ở nền.
func (r *Runner) Start(projectID, extra string) (*store.Session, error) {
	p, ok := r.st.Project(projectID)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy dự án %q", projectID)
	}
	cfg := r.st.Settings()
	release, err := r.acquireWork(projectID, cfg)
	if err != nil {
		return nil, err
	}
	assets := r.st.AssetsByProject(projectID)
	version := len(r.st.SessionsByProject(projectID)) + 1

	sess := &store.Session{
		ProjectID: projectID,
		Backend:   normalizedBackend(cfg.ClaudeBackend),
		Title:     "Edit: " + p.Name,
		Status:    "running",
	}
	r.st.SaveSession(sess)
	state := r.reserve(sess.ID, release, cfg)
	r.broadcast("session", *sess)

	prompt := BuildPrompt(p, assets, extra, version)
	go r.run(state, sess.ID, projectID, prompt, "")
	return sess, nil
}

// Resume tiếp tục phiên đã có với dặn dò thêm (claude --resume).
func (r *Runner) Resume(sessionID, text string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("nội dung dặn dò trống")
	}
	sess, ok := r.st.Session(sessionID)
	if !ok {
		return fmt.Errorf("không tìm thấy phiên %q", sessionID)
	}
	if sess.ClaudeSessionID == "" {
		return errors.New("phiên chưa có Claude session ID, không thể tiếp tục")
	}
	cfg := r.st.Settings()
	if normalizedBackend(sess.Backend) != normalizedBackend(cfg.ClaudeBackend) {
		return errors.New("phiên này dùng backend Claude khác; hãy chọn lại backend cũ hoặc tạo phiên mới")
	}
	_, ok = r.st.Project(sess.ProjectID)
	if !ok {
		return errors.New("dự án không còn tồn tại")
	}
	release, err := r.acquireWork(sess.ProjectID, cfg)
	if err != nil {
		return err
	}
	state := r.reserve(sessionID, release, cfg)
	r.addEvent(&store.SessionEvent{SessionID: sessionID, Type: "user", Payload: text})

	go r.run(state, sessionID, sess.ProjectID, text, sess.ClaudeSessionID)
	return nil
}

// Stop dừng tiến trình claude của phiên, đặt trạng thái "stopped".
func (r *Runner) Stop(sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	sess, ok := r.st.Session(sessionID)
	if !ok {
		return fmt.Errorf("không tìm thấy phiên %q", sessionID)
	}
	state, running := r.active[sessionID]
	if !running || sess.Status != "running" {
		return fmt.Errorf("phiên %q không có tiến trình đang chạy", sessionID)
	}

	// Đánh dấu stopped TRƯỚC khi kill để goroutine đọc stream không ghi đè thành "error".
	sess.Status = "stopped"
	sess.EndedAt = time.Now()
	r.st.SaveSession(&sess)
	r.broadcast("session", sess)

	state.cancel()
	return nil
}

// buildCmd dựng lệnh claude CLI theo Settings; cwd = data/projects/<projectID>.
func (r *Runner) buildCmd(projectID, prompt, resumeID string) (*exec.Cmd, io.ReadCloser, *bytes.Buffer, error) {
	return r.buildCmdWithSettings(projectID, prompt, resumeID, r.st.Settings())
}

func (r *Runner) buildCmdWithSettings(projectID, prompt, resumeID string, cfg store.Settings) (*exec.Cmd, io.ReadCloser, *bytes.Buffer, error) {
	if cfg.ClaudeBackend == "sdk" {
		return r.buildSDKCmdWithSettings(projectID, prompt, resumeID, cfg)
	}
	bin := cfg.ClaudeBin
	if bin == "" {
		bin = "claude"
	}
	args := []string{
		"-p", "--output-format", "stream-json", "--verbose", "--safe-mode",
		"--permission-mode", "dontAsk",
		"--allowedTools", "Read,Write,Edit,Glob,Grep,Bash(ffmpeg *),Bash(ffprobe *),Bash(mkdir *),Bash(cp *),Bash(mv *)",
		"--disallowedTools", "WebFetch,WebSearch",
	}
	if resumeID != "" {
		args = append(args, "--resume", resumeID)
	}

	dir := filepath.Join(r.dataDir, "projects", projectID)
	for _, sub := range []string{"assets", "outputs", "tmp"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, nil, nil, fmt.Errorf("không tạo được thư mục dự án: %w", err)
		}
	}

	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = safeAgentEnv(os.Environ())
	cmd.Stdin = strings.NewReader(prompt)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("không mở được stdout pipe: %w", err)
	}
	return cmd, stdout, &errBuf, nil
}

func (r *Runner) acquireWork(projectID string, cfg store.Settings) (func(), error) {
	release, ok := r.st.ProjectWork.TryAcquire(projectID)
	if !ok {
		return nil, fmt.Errorf("%w: dự án đang có phiên AI hoặc tác vụ xử lý; vui lòng chờ hoàn tất", ErrBusy)
	}
	if normalizedBackend(cfg.ClaudeBackend) != "sdk" {
		return release, nil
	}
	releaseRuntime, ok := agentsdk.TryUseRuntime(r.dataDir)
	if !ok {
		release()
		return nil, fmt.Errorf("%w: Claude Agent SDK đang cài hoặc cập nhật; vui lòng đợi hoàn tất", ErrBusy)
	}
	return func() { releaseRuntime(); release() }, nil
}

func normalizedBackend(value string) string {
	if value == "sdk" {
		return "sdk"
	}
	return "cli"
}

func safeAgentEnv(env []string) []string {
	allowed := map[string]bool{
		"PATH": true, "HOME": true, "USERPROFILE": true, "LOCALAPPDATA": true,
		"APPDATA": true, "SYSTEMROOT": true, "WINDIR": true, "COMSPEC": true,
		"PATHEXT": true, "TEMP": true, "TMP": true, "TMPDIR": true,
		"LANG": true, "LC_ALL": true, "NO_COLOR": true,
	}
	out := make([]string, 0, len(env))
	for _, item := range env {
		name := item
		if i := strings.IndexByte(item, '='); i >= 0 {
			name = item[:i]
		}
		if allowed[strings.ToUpper(name)] {
			out = append(out, item)
		}
	}
	return out
}

// readStream retains result claims but never finalizes a run before Wait.
func (r *Runner) readStream(sessionID, projectID string, out io.Reader) (*streamEvent, error) {
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 64*1024), 10*1024*1024)
	var result *streamEvent
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		if ev := r.handleLine(sessionID, projectID, line); ev != nil {
			if result != nil {
				return result, errors.New("Claude trả nhiều kết quả cuối trong cùng lượt chạy")
			}
			result = ev
		}
	}
	return result, sc.Err()
}

// addEvent lưu event phiên và phát SSE "session_event".
func (r *Runner) addEvent(e *store.SessionEvent) {
	r.st.AddEvent(e)
	r.broadcast("session_event", map[string]any{"sessionId": e.SessionID, "event": *e})
}
