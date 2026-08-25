package setup

import (
	"bufio"
	"context"
	"embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

//go:embed scripts/*.sh scripts/*.ps1
var scriptFS embed.FS

// scriptFile đọc một script cài đặt đã nhúng trong binary.
func scriptFile(name string) ([]byte, error) {
	b, err := scriptFS.ReadFile("scripts/" + name)
	if err != nil {
		return nil, fmt.Errorf("bản dựng này thiếu script %s: %w", name, err)
	}
	return b, nil
}

// maxLine chặn một dòng output khổng lồ (thanh tiến trình pip không xuống dòng)
// làm phình bộ nhớ và làm nghẹn SSE.
const maxLine = 2000

// Run chạy lần lượt mọi bước của plan, gọi onLine cho từng dòng output.
//
// stdout và stderr gộp làm một: trình cài đặt viết tiến trình ra stderr là
// chuyện thường (pip, brew đều thế), tách ra chỉ khiến người dùng thấy màn hình
// trống trong lúc chờ.
func Run(ctx context.Context, p *Plan, onLine func(string)) error {
	defer func() {
		for _, f := range p.Cleanup {
			_ = os.Remove(f)
		}
	}()

	for i, s := range p.Steps {
		if len(p.Steps) > 1 {
			onLine(fmt.Sprintf("▸ Bước %d/%d — %s", i+1, len(p.Steps), s.Label))
		}
		if err := runStep(ctx, s, onLine); err != nil {
			return err
		}
	}
	return nil
}

func runStep(ctx context.Context, s Step, onLine func(string)) error {
	cmd := exec.CommandContext(ctx, s.Bin, s.Args...)
	prepareProcessTree(cmd)
	cmd.Env = append(safeInstallerEnv(os.Environ()), s.Env...)
	// Không có TTY: bắt các trình cài đặt bỏ tô màu và thanh tiến trình động,
	// nếu không người dùng nhận được một mớ mã ANSI trong ô nhật ký.
	cmd.Env = append(cmd.Env,
		"NO_COLOR=1", "TERM=dumb", "PYTHONUNBUFFERED=1",
		"PIP_PROGRESS_BAR=off", "PIP_DISABLE_PIP_VERSION_CHECK=1",
		"HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_COLOR=1", "HOMEBREW_NO_ENV_HINTS=1",
		"DEBIAN_FRONTEND=noninteractive")

	// Một ống chung cho stdout lẫn stderr: giữ đúng thứ tự các dòng như khi chạy
	// trong terminal, và chỉ cần một goroutine đọc nên không phải khoá onLine.
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		_ = pw.Close()
		_ = pr.Close()
		return startErr(s, err)
	}
	tree, err := attachProcessTree(cmd.Process)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = pw.Close()
		_ = pr.Close()
		return fmt.Errorf("không tạo được nhóm tiến trình an toàn cho bộ cài: %w", err)
	}
	defer tree.close()
	processDone := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			tree.terminate()
		case <-processDone:
		}
	}()
	done := make(chan struct{})
	go func() { defer close(done); pump(pr, onLine) }()

	err = cmd.Wait()
	close(processDone)
	<-watcherDone
	_ = pw.Close() // báo hết dữ liệu để pump thoát
	<-done         // đợi đọc nốt, tránh mất mấy dòng cuối (thường là dòng lỗi)
	_ = pr.Close()

	if err != nil {
		return fmt.Errorf("%s thất bại: %w", s.Bin, err)
	}
	return nil
}

// safeInstallerEnv chỉ truyền biến runtime/mạng cần cho trình cài. Allowlist
// quan trọng hơn blacklist: PAT/JWT hoặc credential mới thêm vào môi trường
// sau này cũng không vô tình lọt sang lifecycle script của package bên thứ ba.
func safeInstallerEnv(env []string) []string {
	allowed := map[string]bool{
		"PATH": true, "HOME": true, "USERPROFILE": true, "HOMEDRIVE": true, "HOMEPATH": true,
		"LOCALAPPDATA": true, "APPDATA": true, "PROGRAMDATA": true,
		"PROGRAMFILES": true, "PROGRAMFILES(X86)": true,
		"SYSTEMROOT": true, "WINDIR": true, "COMSPEC": true, "PATHEXT": true,
		"TEMP": true, "TMP": true, "TMPDIR": true, "LANG": true, "LC_ALL": true,
		"HTTP_PROXY": true, "HTTPS_PROXY": true, "ALL_PROXY": true, "NO_PROXY": true,
		"SSL_CERT_FILE": true, "SSL_CERT_DIR": true, "REQUESTS_CA_BUNDLE": true, "CURL_CA_BUNDLE": true,
	}
	out := make([]string, 0, len(env))
	for _, item := range env {
		name := item
		if i := strings.IndexByte(item, '='); i >= 0 {
			name = item[:i]
		}
		u := strings.ToUpper(name)
		if !allowed[u] {
			continue
		}
		out = append(out, item)
	}
	return out
}

// startErr biến "executable file not found" thành câu người dùng hiểu được.
func startErr(s Step, err error) error {
	if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "executable") {
		return fmt.Errorf("không tìm thấy lệnh %q trên máy — %w", s.Bin, err)
	}
	return fmt.Errorf("chạy %s thất bại: %w", s.Bin, err)
}

// pump đọc từng dòng và đẩy sang onLine. Dùng ReadString thay Scanner để không
// vỡ khi gặp dòng dài hơn buffer mặc định của Scanner (64 KB).
func pump(r io.Reader, onLine func(string)) {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if s := clean(line); s != "" {
			onLine(s)
		}
		if err != nil {
			return
		}
	}
}

func clean(s string) string {
	// \r là ký tự thanh tiến trình ghi đè tại chỗ — giữ đoạn cuối cùng.
	if i := strings.LastIndexByte(s, '\r'); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimRight(s, "\r\n \t")
	if len(s) > maxLine {
		s = s[:maxLine] + "…"
	}
	return redactInstallerLine(s)
}

var installerRedactions = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?i)(https?://)[^/@\s:]+:[^/@\s]+@`), `${1}***:***@`},
	{regexp.MustCompile(`(?i)((?:api[_-]?key|token|password)=)[^&\s]+`), `${1}***`},
	{regexp.MustCompile(`(?i)sk-ant-[a-z0-9_-]{8,}`), `sk-ant-***`},
}

func redactInstallerLine(line string) string {
	for _, rule := range installerRedactions {
		line = rule.re.ReplaceAllString(line, rule.repl)
	}
	return line
}
