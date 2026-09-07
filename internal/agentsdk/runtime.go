// Package agentsdk runs the official Python Agent SDK in an isolated venv.
// CLI remains the default; SDK use requires an explicitly configured API key.
package agentsdk

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const Version = "0.2.152"

//go:embed bridge.py
var bridge string

func PythonPath(dataDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(dataDir, "agent-sdk", "venv", "Scripts", "python.exe")
	}
	return filepath.Join(dataDir, "agent-sdk", "venv", "bin", "python")
}

// Check imports the installed library and probes its bundled CLI; never calls AI.
func Check(ctx context.Context, dataDir string) error {
	if _, err := os.Stat(PythonPath(dataDir)); err != nil {
		return fmt.Errorf("chưa cài Claude Agent SDK; cài Python rồi bấm Cài tại Công cụ trên máy")
	}
	release, ok := TryUseRuntime(dataDir)
	if !ok {
		return fmt.Errorf("đang cài hoặc cập nhật Agent SDK; chờ hoàn tất")
	}
	defer release()
	code := `import importlib.metadata as m, pathlib, subprocess, claude_agent_sdk; assert m.version('claude-agent-sdk') == '` + Version + `'; p=pathlib.Path(claude_agent_sdk.__file__).parent/'_bundled'; cli=next(p.glob('claude*')); subprocess.run([str(cli), '--version'], check=True, timeout=8, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL); print('ready')`
	cmd := pythonCommand(ctx, dataDir, "-I", "-c", code)
	cmd.Env = safeEnv(os.Environ())
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "ready" {
		return fmt.Errorf("Agent SDK chưa sẵn sàng; cài Python rồi bấm Cài Claude Agent SDK tại Công cụ trên máy")
	}
	return nil
}

func Budget(value float64) (float64, error) {
	if value == 0 {
		return 2, nil
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0.1 || value > 100 {
		return 0, fmt.Errorf("ngân sách SDK phải từ 0.1 đến 100 USD mỗi lượt")
	}
	return value, nil
}

// Command carries credentials over stdin, never command arguments or disk.
func Command(dataDir, cwd, prompt, resume, apiKey string, budget float64) (*exec.Cmd, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("chế độ Agent SDK cần Anthropic API key riêng; không dùng đăng nhập Claude CLI")
	}
	budget, err := Budget(budget)
	if err != nil {
		return nil, err
	}
	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		return nil, err
	}
	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(PythonPath(dataDir)); err != nil {
		return nil, fmt.Errorf("chưa cài Claude Agent SDK — mở Công cụ trên máy và bấm Cài")
	}
	configDir := filepath.Join(dataDir, "agent-sdk", "config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return nil, fmt.Errorf("không tạo được thư mục phiên SDK: %w", err)
	}
	body, err := json.Marshal(map[string]any{
		"prompt": prompt, "cwd": cwd, "resume": resume, "api_key": apiKey,
		"budget": budget, "config_dir": configDir,
	})
	if err != nil {
		return nil, err
	}
	cmd := pythonCommand(context.Background(), dataDir, "-I", "-u", "-c", bridge)
	cmd.Dir = cwd
	cmd.Env = safeEnv(os.Environ())
	cmd.Stdin = bytes.NewReader(body)
	return cmd, nil
}

func pythonCommand(ctx context.Context, dataDir string, args ...string) *exec.Cmd {
	py := PythonPath(dataDir)
	// Match the installer even when an Intel app is running under Rosetta.
	if runtime.GOOS == "darwin" {
		out, _ := exec.CommandContext(ctx, "/usr/sbin/sysctl", "-n", "hw.optional.arm64").Output()
		if strings.TrimSpace(string(out)) == "1" {
			return exec.CommandContext(ctx, "/usr/bin/arch", append([]string{"-arm64", py}, args...)...)
		}
	}
	return exec.CommandContext(ctx, py, args...)
}

func safeEnv(env []string) []string {
	allow := map[string]bool{"PATH": true, "HOME": true, "USERPROFILE": true, "LOCALAPPDATA": true,
		"APPDATA": true, "SYSTEMROOT": true, "WINDIR": true, "COMSPEC": true, "PATHEXT": true,
		"TEMP": true, "TMP": true, "TMPDIR": true, "LANG": true, "LC_ALL": true}
	var out []string
	for _, item := range env {
		key, _, _ := strings.Cut(item, "=")
		if allow[strings.ToUpper(key)] {
			out = append(out, item)
		}
	}
	return append(out, "PYTHONUTF8=1", "PYTHONIOENCODING=utf-8")
}
