// Package agent — Claude CLI session runner cho tính năng "Phiên AI" edit video.
// Chạy claude CLI (subscription, không API key) trong thư mục dự án, parse
// stream-json và phát sự kiện SSE qua broadcast.
package agent

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bizstudio/internal/store"
)

// Runner quản lý các phiên AI (Claude CLI) đang chạy.
type Runner struct {
	st        *store.Store
	broadcast func(string, any)
	dataDir   string
	mu        sync.Mutex
	procs     map[string]*exec.Cmd
}

// New tạo Runner mới.
func New(st *store.Store, broadcast func(string, any), dataDir string) *Runner {
	return &Runner{
		st:        st,
		broadcast: broadcast,
		dataDir:   dataDir,
		procs:     map[string]*exec.Cmd{},
	}
}

// Start tạo phiên AI mới cho dự án và chạy Claude CLI ở nền.
func (r *Runner) Start(projectID, extra string) (*store.Session, error) {
	p, ok := r.st.Project(projectID)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy dự án %q", projectID)
	}
	assets := r.st.AssetsByProject(projectID)
	version := len(r.st.SessionsByProject(projectID)) + 1

	sess := &store.Session{
		ProjectID: projectID,
		Title:     "Edit: " + p.Name,
		Status:    "running",
	}
	r.st.SaveSession(sess)
	r.broadcast("session", *sess)

	p.Status = "running"
	if p.Progress < 1 {
		p.Progress = 1
	}
	r.st.SaveProject(&p)
	r.broadcast("project", p)

	prompt := BuildPrompt(p, assets, extra, version)
	go r.run(sess.ID, projectID, prompt, "")
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
	r.mu.Lock()
	_, running := r.procs[sessionID]
	r.mu.Unlock()
	if running {
		return errors.New("phiên đang chạy, vui lòng đợi hoàn tất")
	}

	sess.Status = "running"
	r.st.SaveSession(&sess)
	r.broadcast("session", sess)
	r.addEvent(&store.SessionEvent{SessionID: sessionID, Type: "user", Payload: text})

	go r.run(sessionID, sess.ProjectID, text, sess.ClaudeSessionID)
	return nil
}

// Stop dừng tiến trình claude của phiên, đặt trạng thái "stopped".
func (r *Runner) Stop(sessionID string) error {
	sess, ok := r.st.Session(sessionID)
	if !ok {
		return fmt.Errorf("không tìm thấy phiên %q", sessionID)
	}
	r.mu.Lock()
	cmd, running := r.procs[sessionID]
	r.mu.Unlock()
	if !running || cmd.Process == nil {
		return fmt.Errorf("phiên %q không có tiến trình đang chạy", sessionID)
	}

	// Đánh dấu stopped TRƯỚC khi kill để goroutine đọc stream không ghi đè thành "error".
	sess.Status = "stopped"
	sess.EndedAt = time.Now()
	r.st.SaveSession(&sess)
	r.broadcast("session", sess)

	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("không dừng được tiến trình: %w", err)
	}
	return nil
}

// run chạy claude CLI, đọc stream-json và cập nhật phiên khi kết thúc.
func (r *Runner) run(sessionID, projectID, prompt, resumeID string) {
	cmd, stdout, stderr, err := r.buildCmd(projectID, prompt, resumeID)
	if err != nil {
		r.failSession(sessionID, projectID, err.Error())
		return
	}
	if err := cmd.Start(); err != nil {
		r.failSession(sessionID, projectID, "không chạy được claude CLI: "+err.Error())
		return
	}
	r.mu.Lock()
	r.procs[sessionID] = cmd
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.procs, sessionID)
		r.mu.Unlock()
	}()

	gotResult := r.readStream(sessionID, projectID, stdout)
	waitErr := cmd.Wait()
	r.finishRun(sessionID, projectID, gotResult, waitErr, stderr)
}

// buildCmd dựng lệnh claude CLI theo Settings; cwd = data/projects/<projectID>.
func (r *Runner) buildCmd(projectID, prompt, resumeID string) (*exec.Cmd, io.ReadCloser, *bytes.Buffer, error) {
	cfg := r.st.Settings()
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

// readStream đọc từng dòng NDJSON từ stdout của claude. Trả true nếu đã nhận event "result".
func (r *Runner) readStream(sessionID, projectID string, out io.Reader) bool {
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 64*1024), 10*1024*1024)
	gotResult := false
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		if r.handleLine(sessionID, projectID, line) {
			gotResult = true
		}
	}
	if err := sc.Err(); err != nil {
		r.addEvent(&store.SessionEvent{SessionID: sessionID, Type: "error", Payload: "lỗi đọc stream claude: " + err.Error()})
	}
	return gotResult
}

// addEvent lưu event phiên và phát SSE "session_event".
func (r *Runner) addEvent(e *store.SessionEvent) {
	r.st.AddEvent(e)
	r.broadcast("session_event", map[string]any{"sessionId": e.SessionID, "event": *e})
}
