package agentsdk

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInstalledSDKRuntime(t *testing.T) {
	data := os.Getenv("BIZSTUDIO_SDK_TEST_DATA")
	if data == "" {
		t.Skip("optional native installed-SDK smoke; no provider calls")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := Check(ctx, data); err != nil {
		t.Fatal(err)
	}
	// Exercise real stdin/stdout with the same isolated interpreter flags as
	// production. Windows redirected streams otherwise default to an ANSI codepage.
	code := `import json, sys; assert sys.flags.utf8_mode == 1; print(json.dumps(json.load(sys.stdin), ensure_ascii=False))`
	cmd := pythonCommand(ctx, data, "-I", "-u", "-c", code)
	cmd.Env = safeEnv(os.Environ())
	cmd.Stdin = strings.NewReader(`{"text":"Tiếng Việt — dữ liệu 🎬"}`)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("isolated UTF-8 roundtrip: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(out, &got); err != nil || got["text"] != "Tiếng Việt — dữ liệu 🎬" {
		t.Fatalf("SDK corrupted UTF-8 stdin/stdout: %q, %v", out, err)
	}
}

func TestCommandRequiresExplicitKeyAndInstalledRuntime(t *testing.T) {
	dir := t.TempDir()
	if _, err := Command(dir, dir, "render", "", "", 0); err == nil {
		t.Fatal("SDK accepted subscription fallback without an API key")
	}
	if _, err := Command(dir, dir, "render", "", "fake-test-key", 0); err == nil {
		t.Fatal("SDK accepted absent runtime")
	}
	py := PythonPath(dir)
	if err := os.MkdirAll(filepath.Dir(py), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(py, []byte("test fixture, never executed"), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd, err := Command(dir, dir, "render tiếng Việt", "session-old", "fake-test-key", 0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(cmd.Args, " "), "fake-test-key") || strings.Contains(strings.Join(cmd.Env, " "), "fake-test-key") {
		t.Fatal("credential leaked into args or process environment")
	}
	if !strings.Contains(strings.Join(cmd.Args, " "), "-X utf8 -I") {
		t.Fatal("isolated Python must force UTF-8 on the command line; -I ignores PYTHONUTF8")
	}
	body, err := io.ReadAll(cmd.Stdin)
	if err != nil {
		t.Fatal(err)
	}
	var request map[string]any
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatal(err)
	}
	if request["api_key"] != "fake-test-key" || request["resume"] != "session-old" || request["budget"] != float64(2) {
		t.Fatal("SDK stdin protocol changed")
	}
	if _, exists := request["model"]; exists {
		t.Fatal("model must not be pinned")
	}
}

func TestBudgetAndEnvironmentGuards(t *testing.T) {
	for _, bad := range []float64{-1, 0.01, 101, math.Inf(1), math.NaN()} {
		if _, err := Budget(bad); err == nil {
			t.Fatalf("accepted budget %v", bad)
		}
	}
	got := strings.Join(safeEnv([]string{"PATH=/bin", "ANTHROPIC_API_KEY=secret", "CLAUDE_CODE_OAUTH_TOKEN=secret", "ANTHROPIC_BASE_URL=https://evil.invalid", "PYTHONPATH=/evil", "CLAUDE_CONFIG_DIR=/user/config"}), "\n")
	if strings.Contains(got, "secret") || strings.Contains(got, "evil") || strings.Contains(got, "CLAUDE_CONFIG_DIR") {
		t.Fatalf("unsafe environment: %s", got)
	}
}

func TestBridgeOfflineProtocol(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		py, err = exec.LookPath("python")
	}
	if err != nil {
		t.Skip("Python required for offline SDK adapter contract test")
	}
	cmd := exec.Command(py, "-I", "-X", "utf8", "bridge_test.py")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bridge contract: %v\n%s", err, out)
	}
}
